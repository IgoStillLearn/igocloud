**IGO PRIVATE CLOUD**

IGO CLOUD adalah sistem self hosted cloud storage yang memanfaatkan server Telegram (MTProto) sebagai media penyimpanan, dibangun menggunakan arsitektur Golang, SQLite, dan Alpine.js untuk UI dan performa yang ringan serta responsif.

# Fitur utama
1. Unlimited Storage : Menggunakan Telegram Saved Messages sebagai storage filenya
2. Aman dan Privat : Autentikasi menggunakan JWT Cookie dengan konfigurasi kredensial via .env
3. Public File Sharing : share file dengan link tanpa perlu login, link tersebut memiliki masa berlaku(24 jam expired)
5. Docker Ready : untuk production sudah dikontainerisasi penuh untuk proses deployment

# Sistem requirement
1. [Docker](https://www.docker.com/) & Docker Compose terinstal di server/lokal
2. Akun Telegram aktif beserta API ID dan API Hash (buat dapetinnya disini https://my.telegram.org)

# Panduan instalasi (Deployment)
1. Clone Repositori
```bash
git clone [https://github.com/IgoStillLearn/igocloud.git](https://github.com/IgoStillLearn/igocloud.git)
cd igocloud
```
2. sesuaikan .env

   // Konfigurasi Telegram API didapat dari https://my.telegram.org
   TG_API_ID=angka_api_id
   TG_API_HASH=string_api_hash
   
   // Biarkan default 3000 untuk Port
   PORT=3000
   
   // Input nomor telepon
   TG_PHONE=nomor telegram yang digunakan untuk mendapatkan API ID dan Hash(gunakan +628 untuk kode region telepon Indonesia)
   
   // Create random secret key
   JWT_SECRET=bikin_kunci_rahasia_acak_disini
   
   // Konfigurasi Admin (Hanya dipakai saat database masih kosong)
   ADMIN_EMAIL=contohadmin@domain.com
   ADMIN_PASSWORD=contohpassword
   
   // URL
   APP_URL=http://localhost:3000 # Ganti dengan Domain/IP VPS saat production

4. jalankan command
   ```bash
   docker compose up -d --build
   ```
5. akses dashboard : http://localhost:3000





Created by IgoStillLearn - 2026