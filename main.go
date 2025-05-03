package main

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/gorilla/sessions"
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

// Global variables for WhatsApp client and API configuration
var (
	wac           *whatsmeow.Client
	app_host      = "127.0.0.1"
	app_port      = "7069"
	app_key       = "39a473ef08c82eb0cdb13aa28c0b4c9b"
	templates     = template.Must(template.ParseGlob("web/*.html"))
	session_store = sessions.NewCookieStore([]byte(app_key))
	db_users      *sql.DB
)

// Structure for receiving JSON request
type SendMessageRequest struct {
	Number  string `json:"number"`
	Message string `json:"message"`
	ApiKey  string `json:"api_key"`
}

// Function to connect to WhatsApp
func ConnectToWhatsApp() (*whatsmeow.Client, error) {
	// Initialize database storage
	container, err := sqlstore.New("sqlite3", "file:session.lock?_foreign_keys=on", waLog.Noop)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Get the first device store
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		return nil, fmt.Errorf("failed to get device store: %w", err)
	}

	// Create a new WhatsApp client
	client := whatsmeow.NewClient(deviceStore, waLog.Noop)

	// If device ID is nil (not logged in), request QR code
	if client.Store.ID == nil {
		qrChan, err := client.GetQRChannel(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get QR channel: %w", err)
		}

		// Try connecting to WhatsApp
		err = client.Connect()
		if err != nil {
			return nil, fmt.Errorf("failed to connect to WhatsApp: %w", err)
		}

		// Display QR code for login
		for evt := range qrChan {
			if evt.Event == "code" {
				log.Println("Please scan the following QR code to connect to WhatsApp:")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else {
				log.Printf("Login event: %s", evt.Event)
				if evt.Event == "timeout" {
					os.Exit(1)
				}
			}
		}
	} else {
		// If already logged in, connect directly
		err := client.Connect()
		if err != nil {
			return nil, fmt.Errorf("failed to connect to WhatsApp: %w", err)
		}
	}

	return client, nil
}

// Function to generate table for storing user data
func createTable() {
	query := `
    CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        username TEXT UNIQUE,
        password TEXT,
        api_key TEXT
    );
    `
	_, err := db_users.Exec(query)
	if err != nil {
		log.Fatal(err)
	}
}

// Function to create a default admin user if it doesn't exist
func createDefaultAdmin() {
	var count int
	db_users.QueryRow(`SELECT COUNT(*) FROM users WHERE username = 'admin'`).Scan(&count)
	if count == 0 {
		apiKey := generateApiKey()
		password := hashMD5("admin")
		_, err := db_users.Exec(`INSERT INTO users (username, password, api_key) VALUES (?, ?, ?)`, "admin", password, apiKey)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Default user created username: admin | password: admin")
	}
}

// Function to generate a random API key
func generateApiKey() string {
	now := time.Now().String()
	return hashMD5(now)
}

// Function to generate MD5 hash
func hashMD5(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

// Function to validate phone numbers (international format without '+')
func IsValidPhoneNumber(phone string) bool {
	re := regexp.MustCompile(`^(\d{1,3})(\d{6,14})$`)
	return re.MatchString(phone)
}

// Function to handle home page
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "home.html", nil)
}

// Function to handle register page
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := session_store.Get(r, "session")

	if session.Values["user_id"] != nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if username == "" || password == "" {
			data := struct {
				Error string
			}{
				Error: "username and password cannot be empty",
			}

			templates.ExecuteTemplate(w, "register.html", data)
			return
		}

		var exists int
		err := db_users.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, username).Scan(&exists)
		if err != nil {
			data := struct {
				Error string
			}{
				Error: "internal server error",
			}

			templates.ExecuteTemplate(w, "register.html", data)
			return
		}

		if exists > 0 {
			data := struct {
				Error string
			}{
				Error: "username already exists",
			}

			templates.ExecuteTemplate(w, "register.html", data)
			return
		}

		hashedPassword := hashMD5(password)

		apiKey := hashMD5(fmt.Sprintf("%d", time.Now().UnixNano()))

		_, err = db_users.Exec(`INSERT INTO users (username, password, api_key) VALUES (?, ?, ?)`, username, hashedPassword, apiKey)
		if err != nil {
			http.Error(w, "Gagal register", http.StatusInternalServerError)
			return
		}

		var id int
		db_users.QueryRow(`SELECT id FROM users WHERE username = ?`, username).Scan(&id)

		session.Values["user_id"] = id
		session.Save(r, w)

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	templates.ExecuteTemplate(w, "register.html", nil)
}

// Function to handle login page
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := session_store.Get(r, "session")

	if session.Values["user_id"] != nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")
		hashedPassword := hashMD5(password)

		if username == "" || password == "" {
			data := struct {
				Error string
			}{
				Error: "username and password cannot be empty",
			}

			templates.ExecuteTemplate(w, "login.html", data)
			return
		}

		var id int
		var dbPassword string
		err := db_users.QueryRow(`SELECT id, password FROM users WHERE username = ?`, username).Scan(&id, &dbPassword)
		if err != nil || dbPassword != hashedPassword {
			data := struct {
				Error string
			}{
				Error: "Invalid username or password",
			}

			templates.ExecuteTemplate(w, "login.html", data)
			return
		}

		session.Values["user_id"] = id
		session.Save(r, w)

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	templates.ExecuteTemplate(w, "login.html", nil)
}

// Function to handle logout
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := session_store.Get(r, "session")
	session.Values["user_id"] = nil
	session.Options.MaxAge = -1
	session.Save(r, w)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Function to handle dashboard page
func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := session_store.Get(r, "session")
	userID, ok := session.Values["user_id"].(int)
	if !ok || userID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var apiKey string
	err := db_users.QueryRow(`SELECT api_key FROM users WHERE id = ?`, userID).Scan(&apiKey)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	fullURL := fmt.Sprintf("%s://%s", scheme, r.Host)

	data := struct {
		Url    string
		ApiKey string
	}{
		Url:    fullURL,
		ApiKey: apiKey,
	}

	templates.ExecuteTemplate(w, "dashboard.html", data)
}

// Function to handle change credentials
func ChangeCredentialsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	session, _ := session_store.Get(r, "session")
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	newUsername := r.FormValue("newUsername")
	newPassword := r.FormValue("newPassword")

	if newUsername == "" || newPassword == "" {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	var existingID int
	err := db_users.QueryRow("SELECT id FROM users WHERE username = ?", newUsername).Scan(&existingID)

	if err != sql.ErrNoRows && existingID != userID {
		http.Error(w, "Username already taken", http.StatusBadRequest)
		return
	}

	hashedPassword := fmt.Sprintf("%x", md5.Sum([]byte(newPassword)))

	_, err = db_users.Exec("UPDATE users SET username = ?, password = ? WHERE id = ?", newUsername, hashedPassword, userID)
	if err != nil {
		http.Error(w, "Failed to update credentials", http.StatusInternalServerError)
		return
	}

	session.Options.MaxAge = -1
	session.Save(r, w)

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// Function to handle WhatsApp message sending
func SendMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("Method %s not allowed on /send-message", r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "failed",
			"message": "Method " + r.Method + " is not allowed",
		})
		return
	}

	var request SendMessageRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&request)
	if err != nil {
		log.Printf("Failed to parse JSON body: %v", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "failed",
			"message": "Invalid JSON body",
		})
		return
	}

	if request.Number == "" || request.Message == "" || request.ApiKey == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "failed",
			"message": "Number, message, and token cannot be empty",
		})
		return
	}

	var apiKey string
	api_err := db_users.QueryRow(`SELECT api_key FROM users WHERE api_key = ?`, request.ApiKey).Scan(&apiKey)
	if api_err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "failed",
			"message": "Invalid api_key",
		})
		return
	}

	if !IsValidPhoneNumber(request.Number) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "failed",
			"message": "Invalid phone number format",
		})
		return
	}

	if wac == nil {
		log.Println("WhatsApp is not connected")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "failed",
			"message": "WhatsApp is not connected",
		})
		return
	}

	jid := types.JID{
		User:   request.Number,
		Server: types.DefaultUserServer,
	}

	message := &waProto.Message{
		Conversation: proto.String(request.Message),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultChan := make(chan error, 1)
	go func() {
		_, err := wac.SendMessage(ctx, jid, message)
		resultChan <- err
	}()

	select {
	case err := <-resultChan:
		if err != nil {
			log.Printf("Failed to send message to %s: %v", request.Number, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "failed",
				"message": "Failed to send message: " + err.Error(),
			})
			return
		}
	case <-ctx.Done():
		log.Printf("Timeout sending message to %s", request.Number)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestTimeout)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "failed",
			"message": "Failed to send message: timeout",
		})
		return
	}

	log.Printf("Message sent to +%s", request.Number)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Message successfully sent to " + request.Number,
	})
}

func main() {
	log.Println("Starting WhatsApp Message Sender Service")

	err := godotenv.Load("kirimkan.conf")
	if err != nil {
		log.Printf("Failed to read configuration file: %v", err)
		os.Exit(1)
	}

	app_host = os.Getenv("API_HOST")
	app_port = os.Getenv("API_PORT")
	app_key = os.Getenv("API_KEY")

	log.Println("Configuration loaded:")
	log.Printf("API_HOST: %s", app_host)
	log.Printf("API_PORT: %s", app_port)

	session_store = sessions.NewCookieStore([]byte(app_key))

	var errWA error
	wac, errWA = ConnectToWhatsApp()
	if errWA != nil {
		log.Printf("Failed to connect to WhatsApp: %v", errWA)
		return
	} else {
		log.Println("Connected to WhatsApp")
	}

	defer wac.Disconnect()

	var db_user_err error
	db_users, db_user_err = sql.Open("sqlite3", "users.sqlite")
	if db_user_err != nil {
		log.Printf("Failed to connect to database: %v", db_user_err)
		return
	}
	defer db_users.Close()

	createTable()
	createDefaultAdmin()

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", HomeHandler)
	http.HandleFunc("/register", RegisterHandler)
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/logout", LogoutHandler)
	http.HandleFunc("/dashboard", DashboardHandler)
	http.HandleFunc("/change-credentials", ChangeCredentialsHandler)
	http.HandleFunc("/send-message", SendMessageHandler)

	fmt.Print(`
██╗  ██╗    ██╗    ██████╗     ██╗    ███╗   ███╗    ██╗  ██╗     █████╗     ███╗   ██╗
██║ ██╔╝    ██║    ██╔══██╗    ██║    ████╗ ████║    ██║ ██╔╝    ██╔══██╗    ████╗  ██║
█████╔╝     ██║    ██████╔╝    ██║    ██╔████╔██║    █████╔╝     ███████║    ██╔██╗ ██║
██╔═██╗     ██║    ██╔══██╗    ██║    ██║╚██╔╝██║    ██╔═██╗     ██╔══██║    ██║╚██╗██║
██║  ██╗    ██║    ██║  ██║    ██║    ██║ ╚═╝ ██║    ██║  ██╗    ██║  ██║    ██║ ╚████║
╚═╝  ╚═╝    ╚═╝    ╚═╝  ╚═╝    ╚═╝    ╚═╝     ╚═╝    ╚═╝  ╚═╝    ╚═╝  ╚═╝    ╚═╝  ╚═══╝
`)

	addr := fmt.Sprintf("%s:%s", app_host, app_port)
	log.Printf("Server is running on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
