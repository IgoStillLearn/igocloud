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

2. buat file .env atau edit .env.example menjadi .env dan sesuaikan isinya

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

3. jalankan command berikut di terminal :
```bash
go run .
```

4. masukan nomor telepon yang kalian gunakan di telegram pada terminal tersebut

5. masukan OTP yang kalian terima di telegram(ini akan generate folder data/session.json untuk save login session kalian, agar tidak perlu login ulang kedepannya)

6. akses dashboard : http://localhost:3000



notes : di main.go itu ada opsi code untuk fase development dan fase deployment, silahkan enable dan disable "//" sesuai fase
Jika kalian ingin mendeploy ke VPS menggunakan Docker, jangan melakukan login OTP pertama kali di dalam Docker.
Jalankan aplikasi secara lokal terlebih dahulu (go run .) hingga berhasil login. Setelah file session.json terbuat di folder data/, ganti code loginnya di main.go ke code untuk deployment, baru build image docker dan push, baru pull image docker di VPS, dan edit file data/session.json atau kalau belum ada, buat terlebih dahulu coy :
```bash
mkdir -p data && touch data/session.json
```
lalu 
```bash
cd data
```

```bash
nano session.json
```
dan paste session.json di VS code ke session.json yang berada di VPS ini.

Created by IgoStillLearn - 2026