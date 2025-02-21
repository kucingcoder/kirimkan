package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// Variabel global untuk menyimpan koneksi database, WhatsApp client, dan mutex
var (
	db_save_number = "no"
	db_host        = "127.0.0.1"
	db_username    = "root"
	db_password    = ""
	db_database    = "kirimkan"
	dsn            = "root:@tcp(127.0.0.1:3306)/kirimkan"
	db             *sql.DB
	wac            *whatsmeow.Client
	mu             sync.Mutex
	app_host       = "127.0.0.1"
	app_port       = "6969"
)

// Struktur untuk menerima data JSON
type BodyKirimPesan struct {
	No    string `json:"no"`
	Pesan string `json:"pesan"`
}

// Fungsi untuk menghubungkan atau memperbarui koneksi database
func KoneksiDB() {
	mu.Lock()
	defer mu.Unlock()

	// Jika koneksi sudah ada dan masih aktif, tidak perlu membuat koneksi baru
	if db != nil {
		err := db.Ping()
		if err == nil {
			return // Koneksi masih bagus, langsung return
		}
		db.Close()
	}

	// Buat koneksi baru
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("Gagal membuat koneksi ke Database : %v", err)
		return
	}

	// Cek koneksi
	err = db.Ping()
	if err != nil {
		log.Printf("Gagal terhubung ke Database : %v", err)
		db.Close()
		db = nil
		return
	}

	log.Printf("Terhubung ke Database")
}

// Fungsi untuk menghubungkan WhatsApp client
func KoneksiWA() (*whatsmeow.Client, error) {
	// Inisialisasi penyimpanan database
	container, err := sqlstore.New("sqlite3", "file:session.lock?_foreign_keys=on", waLog.Noop)
	if err != nil {
		return nil, fmt.Errorf("gagal menginisialisasi database: %w", err)
	}

	// Mendapatkan device store pertama
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		return nil, fmt.Errorf("gagal mendapatkan device store: %w", err)
	}

	// Membuat WhatsApp client baru
	client := whatsmeow.NewClient(deviceStore, waLog.Noop)

	// Jika ID perangkat belum ada (belum login), dapatkan QR code untuk login
	if client.Store.ID == nil {
		qrChan, err := client.GetQRChannel(context.Background())
		if err != nil {
			return nil, fmt.Errorf("gagal mendapatkan QR channel: %w", err)
		}

		// Mencoba menghubungkan ke WhatsApp
		err = client.Connect()
		if err != nil {
			return nil, fmt.Errorf("gagal menghubungkan ke WhatsApp: %w", err)
		}

		// Menampilkan QR code untuk login
		for evt := range qrChan {
			if evt.Event == "code" {
				log.Printf("Silahkan scan QR code berikut agar terhubung ke Whatsapp")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else {
				log.Printf("Event Login : %s", evt.Event)
			}
		}
	} else {
		// Jika sudah login, langsung hubungkan
		err := client.Connect()
		if err != nil {
			return nil, fmt.Errorf("gagal menghubungkan ke WhatsApp: %w", err)
		}
	}

	return client, nil
}

// Fungsi untuk menyimpan nomor WhatsApp ke database
func SimpanNomor(nomor string) {
	// Cek koneksi database
	KoneksiDB()

	err := db.QueryRow("SELECT id FROM whatsapp WHERE nomor = ?", nomor).Scan(new(int))

	if err == sql.ErrNoRows {
		// Jika nomor belum ada, masukkan ke database
		_, err := db.Exec("INSERT INTO whatsapp (nomor) VALUES (?)", nomor)
		if err != nil {
			log.Printf("Gagal menyimpan nomor : %v", err)
		}
		log.Printf("Nomor +%s berhasil disimpan", nomor)
	} else if err != nil {
		log.Printf("Gagal memeriksa nomor : %v", err)
	}
}

// Fungsi untuk menangani permintaan pengiriman pesan WhatsApp
func KirimPesan(w http.ResponseWriter, r *http.Request) {
	// Pastikan menggunakan metode POST
	if r.Method != http.MethodPost {
		log.Printf("Metode %s tidak diizinkan di /kirim-pesan", r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)

		// Kirim balasan JSON
		response := map[string]interface{}{
			"status":  "failed",
			"message": "Metode " + r.Method + " tidak diizinkan",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Decode body JSON
	var request BodyKirimPesan
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&request)
	if err != nil {
		log.Printf("Gagal membaca body JSON : %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		// Kirim balasan JSON
		response := map[string]interface{}{
			"status":  "failed",
			"message": "JSON body tidak valid",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Periksa apakah nomor atau pesan kosong
	if request.No == "" || request.Pesan == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		// Kirim balasan JSON
		response := map[string]interface{}{
			"status":  "failed",
			"message": "Nilai 'No' atau 'Pesan' tidak boleh kosong",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Jika client WhatsApp belum terhubung, kirimkan error
	if wac == nil {
		log.Printf("WhatsApp belum terhubung")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)

		// Kirim balasan JSON
		response := map[string]interface{}{
			"status":  "failed",
			"message": "WhatsApp belum terhubung",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Membuat JID untuk nomor tujuan
	jid := types.JID{
		User:   request.No,
		Server: types.DefaultUserServer,
	}

	// Membuat pesan yang akan dikirim
	message := &waProto.Message{
		Conversation: proto.String(request.Pesan),
	}

	// Mengirim pesan
	_, err = wac.SendMessage(context.Background(), jid, message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Printf("Gagal mengirim pesan ke %s : %v", request.No, err)
		return
	}

	// Menyimpan nomor WhatsApp ke database
	if db_save_number == "yes" {
		SimpanNomor(request.No)
	}

	// Mengirim status OK jika berhasil
	log.Printf("Mengirim pesan ke +%s\n", request.No)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Kirim balasan JSON
	response := map[string]interface{}{
		"status":  "success",
		"message": "Pesan berhasil dikirim ke " + request.No,
	}
	json.NewEncoder(w).Encode(response)
}

func main() {
	// Mulai layanan
	log.Printf("Kirimkan dijalankan")

	// Load file konfigurasi
	err := godotenv.Load("kirimkan.conf")
	if err != nil {
		log.Printf("Gagal membaca file konfigurasi : %v", err)
	}

	// Mendapatkan nilai konfigurasi
	app_host = os.Getenv("API_HOST")
	app_port = os.Getenv("API_PORT")
	db_save_number = os.Getenv("DB_SAVE_NUMBER")

	// Menampilkan konfigurasi
	log.Printf("Memuat konfigurasi")
	log.Printf("APP_HOST : %s", app_host)
	log.Printf("APP_PORT : %s", app_port)

	if db_save_number == "yes" {
		db_host = os.Getenv("DB_HOST")
		db_username = os.Getenv("DB_USERNAME")
		db_password = os.Getenv("DB_PASSWORD")
		db_database = os.Getenv("DB_DATABASE")

		log.Printf("DB_SAVE_NUMBER : %s", db_save_number)
		log.Printf("DB_HOST : %s", db_host)
		log.Printf("DB_USERNAME : %s", db_username)
		log.Printf("DB_PASSWORD : %s", db_password)
		log.Printf("DB_DATABASE : %s", db_database)

		// Memperbaharui url koneksi database
		dsn = fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true", db_username, db_password, db_host, db_database)

		// Menghubungkan ke Database
		KoneksiDB()

		// Membuat tabel jika belum ada
		query := "CREATE TABLE IF NOT EXISTS whatsapp (id INT PRIMARY KEY AUTO_INCREMENT, nomor VARCHAR(16) UNIQUE NOT NULL);"
		_, err = db.Exec(query)
		if err != nil {
			log.Printf("Gagal membuat tabel 'whatsapp' : %v", err)
			return
		}
		log.Printf("Tabel 'whatsapp' siap digunakan")
	}

	// Menghubungkan ke WhatsApp
	var err_wa error
	wac, err_wa = KoneksiWA()
	if err_wa != nil {
		log.Printf("Gagal Terhubung ke WhatsApp : %v", err_wa)
		return
	} else {
		log.Printf("Terhubung ke WhatsApp")
	}

	// Menutup koneksi WhatsApp saat aplikasi selesai
	defer wac.Disconnect()

	// Menambahkan rute untuk file statik
	http.Handle("/", http.FileServer(http.Dir("web")))

	// Menambahkan rute untuk API
	http.HandleFunc("/kirim-pesan", KirimPesan)

	// Menampilkan teks
	fmt.Print(`
██╗  ██╗    ██╗    ██████╗     ██╗    ███╗   ███╗    ██╗  ██╗     █████╗     ███╗   ██╗
██║ ██╔╝    ██║    ██╔══██╗    ██║    ████╗ ████║    ██║ ██╔╝    ██╔══██╗    ████╗  ██║
█████╔╝     ██║    ██████╔╝    ██║    ██╔████╔██║    █████╔╝     ███████║    ██╔██╗ ██║
██╔═██╗     ██║    ██╔══██╗    ██║    ██║╚██╔╝██║    ██╔═██╗     ██╔══██║    ██║╚██╗██║
██║  ██╗    ██║    ██║  ██║    ██║    ██║ ╚═╝ ██║    ██║  ██╗    ██║  ██║    ██║ ╚████║
╚═╝  ╚═╝    ╚═╝    ╚═╝  ╚═╝    ╚═╝    ╚═╝     ╚═╝    ╚═╝  ╚═╝    ╚═╝  ╚═╝    ╚═╝  ╚═══╝
`)

	// Menjalankan server API
	log.Printf("Server API berjalan di http://%s:%s\n", app_host, app_port)
	log.Fatal(http.ListenAndServe(app_host+":"+app_port, nil))
}
