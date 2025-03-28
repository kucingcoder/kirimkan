package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

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

// Variabel global untuk menyimpan WhatsApp client dan konfigurasi API
var (
	wac       *whatsmeow.Client
	app_host  = "127.0.0.1"
	app_port  = "7069"
	app_token = "983be0d548d7936377fd9f6279ae7c4f"
)

// Struktur untuk menerima data JSON
type BodyKirimPesan struct {
	No    string `json:"no"`
	Pesan string `json:"pesan"`
	Token string `json:"token"`
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
				if evt.Event == "timeout" {
					os.Exit(1)
				}
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

// Fungsi untuk mengecek apakah nomor telepon sesuai format internasional tanpa tanda "+"
func NomorValid(phone string) bool {
	// Regex untuk validasi nomor dengan kode negara (1-3 digit) diikuti oleh nomor telepon (minimal 6-14 digit)
	re := regexp.MustCompile(`^(\d{1,3})(\d{6,14})$`)
	return re.MatchString(phone)
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
	if request.No == "" || request.Pesan == "" || request.Token == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		// Kirim balasan JSON
		response := map[string]interface{}{
			"status":  "failed",
			"message": "Nomor, pesan, dan token tidak boleh kosong",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Periksa apakah token sesuai
	if request.Token != app_token {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		// Kirim balasan JSON
		response := map[string]interface{}{
			"status":  "failed",
			"message": "Token tidak valid",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Periksa apakah nomor valid
	if !NomorValid(request.No) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		// Kirim balasan JSON
		response := map[string]interface{}{
			"status":  "failed",
			"message": "Nomor tidak valid",
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

	// Buat context dengan timeout 5 detik
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Channel untuk menerima hasil pengiriman
	resultChan := make(chan error, 1)

	// Jalankan pengiriman dalam goroutine
	go func() {
		_, err := wac.SendMessage(ctx, jid, message)
		resultChan <- err
	}()

	// Menunggu hasil pengiriman atau timeout
	select {
	case err := <-resultChan:
		if err != nil {
			log.Printf("Gagal mengirim pesan ke %s: %v", request.No, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "failed",
				"message": "Gagal mengirim pesan: " + err.Error(),
			})
			return
		}
	case <-ctx.Done():
		log.Printf("Gagal mengirim pesan ke %s: Waktu habis", request.No)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestTimeout)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "failed",
			"message": "Gagal mengirim pesan: Waktu habis",
		})
		return
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
		os.Exit(1)
	}

	// Mendapatkan nilai konfigurasi
	app_host = os.Getenv("API_HOST")
	app_port = os.Getenv("API_PORT")
	app_token = os.Getenv("API_TOKEN")

	// Menampilkan konfigurasi
	log.Printf("Memuat konfigurasi")
	log.Printf("APP_HOST : %s", app_host)
	log.Printf("APP_PORT : %s", app_port)
	log.Printf("APP_TOKEN : %s", app_token)

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
