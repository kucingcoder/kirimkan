![ChatGPT Image May 3, 2025, 09_48_41 PM](https://github.com/user-attachments/assets/3712eb06-b636-41b6-8a34-e8a7e6269dc1)


# kirimkan

A self-hosted WhatsApp messaging API built with Go.  
Send WhatsApp messages via HTTP requests for OTP, notifications, and more.

## ✨ Features

- 📤 Send WhatsApp messages through a REST API
- 🔐 Self-hosted: No third-party service required
- ⚡ Built with Go for performance and simplicity
- ✅ Useful for OTPs, alerts, notifications, and automation

## 📦 PROJECT SETUP

### Prerequisites

- Go 1.23.0 or later
- [Whatsapp App](https://play.google.com/store/apps/details?id=com.whatsapp) for scan QR (connectiong accounts)

### Build

- Clone repo

```bash
git clone https://github.com/kucingcoder/kirimkan.git
```

- Go to repo folder

```bash
cd kirimkan
```

- Install dependency

```bash
go mod tidy
```

- Build binary

```bash
go build
```

## 🔍 USAGE

- Run binary

```bash
./kirimkan
```

- Open the website api in your browser
- Login with default credential

  - username : admin
  - password : admin

- Change admin password for security
- Relogin to make sure was changed
- If you want to share with your friends, there is a registration menu to create a new user.

## ⚙️ CUSTOMIZATION

if you want to change the binding ip address and also the port you can change it in the `kirimkan.conf` file and then restart the application. you can also change the secret key used for session encryption in the API_KEY section because you should not use the default. if you have problems logging back into the account use /logout to delete the old session.
