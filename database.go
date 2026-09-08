package main

import (
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var DB *gorm.DB

type User struct {
	gorm.Model
	Name     string
	Email    string `gorm:"unique"`
	Password string
}

type Folder struct {
	gorm.Model
	Name       string
	ParentID   *uint
	IsFavorite bool `gorm:"default:false"`
}

type File struct {
	gorm.Model
	Name           string
	Size           int64
	Type           string
	FolderID       *uint
	TelegramMsgID  int
	IsFavorite     bool    `gorm:"default:false"`
	ShareHash      *string `gorm:"uniqueIndex"`
	ShareExpiresAt *time.Time
}

func ConnectDB() {

	database, err := gorm.Open(sqlite.Open("igocloud.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("❌ Gagal konek ke Database SQLite:", err)
	}

	err = database.AutoMigrate(&User{}, &Folder{}, &File{})
	if err != nil {
		log.Fatal("❌ Gagal migrasi struktur tabel database:", err)
	}

	DB = database
	log.Println("✅ [DATABASE] SQLite berhasil terhubung dan struktur tabel siap!")

	SeedAdmin()
}

func SeedAdmin() {
	var admin User

	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPass := os.Getenv("ADMIN_PASSWORD")

	if adminEmail == "" {
		adminEmail = "admin@igocloud.my.id"
	}
	if adminPass == "" {
		adminPass = "rahasia"
	}

	result := DB.Where("email = ?", adminEmail).First(&admin)

	if result.Error != nil {

		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(adminPass), 14)

		admin = User{
			Name:     "Admin IGO CLOUD",
			Email:    adminEmail,
			Password: string(hashedPassword),
		}

		DB.Create(&admin)
		log.Println("✅ [SYSTEM] Akun Admin default berhasil dibuat dari .env!")
	}
}
