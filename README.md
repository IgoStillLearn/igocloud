# ☁️ IGO PRIVATE CLOUD

IGO Cloud is a lightweight, self-hosted cloud storage solution that ingeniously leverages Telegram's MTProto (Saved Messages) as its backend storage. Built with a modern stack featuring Golang, SQLite, and Alpine.js, it delivers a blazing-fast, responsive, and resource-friendly user experience.

##  Key Features
- **🚀 Unlimited Storage:** Utilizes Telegram's Saved Messages to give you virtually boundless storage space.
- **🔒 Secure & Private:** Secured by JWT Cookie authentication with easy and flexible credential configuration via `.env`.
- **🔗 Public File Sharing:** Generate shareable links that don't require user login. For security, these links automatically expire after 24 hours.
- **🐳 Docker Ready:** Fully containerized for a seamless and hassle-free production deployment.

## 🛠️ System Requirements
1. **[Docker](https://www.docker.com/) & Docker Compose** installed on your server or local machine.
2. An active **Telegram Account** along with its `API ID` and `API Hash`. (You can get yours at [my.telegram.org](https://my.telegram.org)).
3. **Go** installed (for local initialization).

## 🚀 Installation & Deployment Guide

### 1. Clone the Repository
```bash
git clone https://github.com/IgoStillLearn/igocloud.git
cd igocloud
```

### 2. Environment Setup
Create a `.env` file or duplicate `.env.example` and fill in your configurations:

```env
# Telegram API Credentials (from https://my.telegram.org)
TG_API_ID=your_api_id_here
TG_API_HASH=your_api_hash_here

# Port Configuration
PORT=3000

# Telegram Phone Number (Use international format, e.g., +628...)
TG_PHONE=+628...

# JWT Secret (Generate a strong random string here)
JWT_SECRET=your_random_secret_key_here

# Initial Admin Config (Used only when the database is empty)
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=your_secure_password

# App URL (Change to your VPS Domain/IP for production)
APP_URL=http://localhost:3000
```

### 3. Local Initialization (Crucial Step)
Run the application locally first to generate the session file:
```bash
go run .
```

### 4. OTP Authentication
- Enter the phone number associated with your Telegram account in the terminal.
- Input the **OTP** sent to your Telegram app.
- Once authenticated, the app will generate a `data/session.json` file to save your login session, preventing the need for repeated logins.

### 5. Access Dashboard
You can now access your local dashboard at: `http://localhost:3000`

---

## ⚠️ Important Deployment Notes (VPS & Docker Workflow)

In `main.go`, you will find specific code blocks for the **Development Phase** and **Deployment Phase**. Please comment (`//`) or uncomment these sections according to your current phase.

**Crucial Docker Rule:** 
Do **NOT** perform your first OTP login inside a Docker container. Follow this exact workflow for VPS deployment:

1. **Run Locally First:** Execute `go run .` locally until you successfully log in and `data/session.json` is generated.
2. **Switch to Deployment Mode:** Update the code in `main.go` to activate the deployment phase configuration.
3. **Build & Push:** Build your Docker image and push it to your registry.
4. **Pull & Setup VPS:** Pull the Docker image on your VPS.
5. **Transfer Session Data:** Manually copy your local `session.json` to the VPS. If the folder doesn't exist, create it:
   ```bash
   mkdir -p data && touch data/session.json
   cd data
   nano session.json
   ```
   *(Paste the contents of your local `session.json` from VS Code into the VPS `session.json` file and save).*

---
*Created by Ade-IgoStillLearn - 2026*