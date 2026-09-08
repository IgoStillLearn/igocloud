package main

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
)

var jwtSecret []byte

func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		cookie := c.Cookies("igo_auth")
		if cookie == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Akses Ditolak"})
		}
		token, err := jwt.Parse(cookie, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Sesi tidak valid"})
		}
		return c.Next()
	}
}

func getFileType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg":
		return "photo"
	case ".mp4", ".mkv", ".avi", ".mov", ".webm":
		return "video"
	case ".apk", ".exe", ".msi", ".dmg":
		return "app"
	default:
		return "doc"
	}
}

func main() {
	_ = godotenv.Load()

	ConnectDB()

	var count int64
	DB.Model(&User{}).Count(&count)
	if count == 0 {
		adminEmail := os.Getenv("ADMIN_EMAIL")
		adminPass := os.Getenv("ADMIN_PASSWORD")

		if adminEmail == "" {
			adminEmail = "admin@igocloud.my.id"
		}
		if adminPass == "" {
			adminPass = "rahasia"
		}

		hash, _ := bcrypt.GenerateFromPassword([]byte(adminPass), 14)
		admin := User{
			Name:     "Admin IGO CLOUD",
			Email:    adminEmail,
			Password: string(hash),
		}
		DB.Create(&admin)
		log.Println("✅ Akun Admin berhasil dibuat dari brankas .env!")
	}

	apiID, _ := strconv.Atoi(os.Getenv("TG_API_ID"))
	apiHash := os.Getenv("TG_API_HASH")
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	ctx := context.Background()
	sessionStorage := &session.FileStorage{Path: "data/session.json"}

	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		SessionStorage: sessionStorage,
	})

	err := client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil || !status.Authorized {
			log.Fatal("❌ Akses Ditolak: Telegram sesi tidak valid.")
		}
		log.Println("✅ [SECURED] Berhasil masuk ke Telegram Cloud!")

		api := client.API()
		up := uploader.NewUploader(api)
		sender := message.NewSender(api).WithUploader(up)
		go func() {
			for {
				time.Sleep(24 * time.Hour)
				log.Println("🧹 Menjalankan pembersihan Trash otomatis...")

				thirtyDaysAgo := time.Now().Add(-30 * 24 * time.Hour)
				var expiredFiles []File
				DB.Unscoped().Where("deleted_at < ?", thirtyDaysAgo).Find(&expiredFiles)

				for _, f := range expiredFiles {
					if f.TelegramMsgID != 0 {
						api.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
							ID:     []int{f.TelegramMsgID},
							Revoke: true,
						})
					}
					// 2. Hancurkan dari SQLite
					DB.Unscoped().Delete(&f)
					log.Printf("🗑️ [AUTO-DELETE] File '%s' dihancurkan permanen (Lewat 30 hari)\n", f.Name)
				}
			}
		}()

		app := fiber.New(fiber.Config{BodyLimit: 2 * 1024 * 1024 * 1024})
		allowedOrigin := os.Getenv("APP_URL")
		if allowedOrigin == "" {
			allowedOrigin = "http://localhost:3000"
		}

		app.Use(cors.New(cors.Config{
			AllowOrigins:     allowedOrigin,
			AllowCredentials: true,
		}))

		app.Static("/", "./public")

		app.Post("/api/login", func(c *fiber.Ctx) error {
			type LoginRequest struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			var req LoginRequest
			c.BodyParser(&req)

			var user User
			if err := DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
				return c.Status(401).JSON(fiber.Map{"error": "Email tidak terdaftar"})
			}
			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
				return c.Status(401).JSON(fiber.Map{"error": "Password salah!"})
			}

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"user_id": user.ID,
				"exp":     time.Now().Add(time.Hour * 72).Unix(),
			})
			t, _ := token.SignedString(jwtSecret)

			c.Cookie(&fiber.Cookie{
				Name: "igo_auth", Value: t, Expires: time.Now().Add(time.Hour * 72),
				HTTPOnly: true, SameSite: "Lax",
			})
			return c.JSON(fiber.Map{"status": "success", "user": fiber.Map{"name": user.Name, "email": user.Email}})
		})

		app.Post("/api/logout", func(c *fiber.Ctx) error {
			c.Cookie(&fiber.Cookie{Name: "igo_auth", Value: "", Expires: time.Now().Add(-time.Hour), HTTPOnly: true})
			return c.JSON(fiber.Map{"status": "success"})
		})
		apiGroup := app.Group("/api", Protected())
		apiGroup.Post("/folders", func(c *fiber.Ctx) error {
			var req struct {
				Name     string `json:"name"`
				ParentID *uint  `json:"parent_id"`
			}
			if err := c.BodyParser(&req); err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "Format salah"})
			}

			folder := Folder{Name: req.Name, ParentID: req.ParentID}
			DB.Create(&folder)
			return c.JSON(fiber.Map{"status": "success", "data": folder})
		})

		apiGroup.Get("/folders", func(c *fiber.Ctx) error {
			var folders []Folder
			DB.Find(&folders)
			return c.JSON(fiber.Map{"status": "success", "data": folders})
		})

		apiGroup.Put("/file/:id/favorite", func(c *fiber.Ctx) error {
			id := c.Params("id")
			var file File
			if err := DB.First(&file, id).Error; err != nil {
				return c.Status(404).JSON(fiber.Map{"error": "File tidak ditemukan"})
			}

			file.IsFavorite = !file.IsFavorite
			DB.Save(&file)
			return c.JSON(fiber.Map{"status": "success"})
		})

		apiGroup.Put("/bulk/favorite", func(c *fiber.Ctx) error {
			var req struct {
				FileIDs   []string `json:"file_ids"`
				FolderIDs []string `json:"folder_ids"`
				IsFav     bool     `json:"is_favorite"`
			}
			if err := c.BodyParser(&req); err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "Format salah"})
			}
			var parsedFileIDs []int
			for _, idStr := range req.FileIDs {
				id, _ := strconv.Atoi(idStr)
				parsedFileIDs = append(parsedFileIDs, id)
			}

			var parsedFolderIDs []int
			for _, idStr := range req.FolderIDs {
				id, _ := strconv.Atoi(idStr)
				parsedFolderIDs = append(parsedFolderIDs, id)
			}
			if len(parsedFileIDs) > 0 {
				DB.Model(&File{}).Where("id IN ?", parsedFileIDs).Update("is_favorite", req.IsFav)
			}
			if len(parsedFolderIDs) > 0 {
				DB.Model(&Folder{}).Where("id IN ?", parsedFolderIDs).Update("is_favorite", req.IsFav)
			}

			return c.JSON(fiber.Map{"status": "success"})
		})

		apiGroup.Put("/files/move", func(c *fiber.Ctx) error {
			var req struct {
				FileIDs  []int `json:"file_ids"`
				FolderID *uint `json:"folder_id"`
			}
			if err := c.BodyParser(&req); err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "Format salah"})
			}
			DB.Model(&File{}).Where("id IN ?", req.FileIDs).Update("folder_id", req.FolderID)
			return c.JSON(fiber.Map{"status": "success"})
		})

		apiGroup.Put("/folders/:id/favorite", func(c *fiber.Ctx) error {
			id := c.Params("id")
			var folder Folder
			if err := DB.First(&folder, id).Error; err != nil {
				return c.Status(404).JSON(fiber.Map{"error": "Folder tidak ditemukan"})
			}
			folder.IsFavorite = !folder.IsFavorite
			DB.Save(&folder)
			return c.JSON(fiber.Map{"status": "success"})
		})

		apiGroup.Delete("/folders/:id", func(c *fiber.Ctx) error {
			id := c.Params("id")
			var folder Folder
			if err := DB.First(&folder, id).Error; err != nil {
				return c.Status(404).JSON(fiber.Map{"error": "Not found"})
			}

			DB.Model(&File{}).Where("folder_id = ?", id).Update("folder_id", folder.ParentID)
			DB.Model(&Folder{}).Where("parent_id = ?", id).Update("parent_id", folder.ParentID)

			DB.Delete(&Folder{}, id)
			return c.JSON(fiber.Map{"status": "success"})
		})

		apiGroup.Put("/folders/move", func(c *fiber.Ctx) error {
			var req struct {
				FolderIDs []int `json:"folder_ids"`
				ParentID  *uint `json:"parent_id"`
			}
			if err := c.BodyParser(&req); err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "Format salah"})
			}
			DB.Model(&Folder{}).Where("id IN ?", req.FolderIDs).Update("parent_id", req.ParentID)
			return c.JSON(fiber.Map{"status": "success"})
		})
		apiGroup.Get("/download/zip", func(c *fiber.Ctx) error {
			fileIDs := c.Query("files")
			folderIDs := c.Query("folders")

			type zipItem struct {
				File    File
				ZipPath string
			}
			var itemsToZip []zipItem

			var getFilesInFolder func(folderID uint, basePath string)
			getFilesInFolder = func(folderID uint, basePath string) {
				var folder Folder
				if err := DB.First(&folder, folderID).Error; err != nil {
					return
				}
				currentPath := basePath + folder.Name + "/"

				var files []File
				DB.Where("folder_id = ?", folderID).Find(&files)
				for _, f := range files {
					itemsToZip = append(itemsToZip, zipItem{File: f, ZipPath: currentPath + f.Name})
				}

				var subfolders []Folder
				DB.Where("parent_id = ?", folderID).Find(&subfolders)
				for _, sf := range subfolders {
					getFilesInFolder(sf.ID, currentPath)
				}
			}

			if fileIDs != "" {
				for _, idStr := range strings.Split(fileIDs, ",") {
					id, _ := strconv.Atoi(idStr)
					var f File
					if err := DB.First(&f, id).Error; err == nil {
						itemsToZip = append(itemsToZip, zipItem{File: f, ZipPath: f.Name})
					}
				}
			}

			if folderIDs != "" {
				for _, idStr := range strings.Split(folderIDs, ",") {
					id, _ := strconv.Atoi(idStr)
					getFilesInFolder(uint(id), "")
				}
			}

			if len(itemsToZip) == 0 {
				return c.Status(400).SendString("Tidak ada file valid yang bisa di-download")
			}

			c.Set("Content-Disposition", "attachment; filename=\"IGO_CLOUD_Download.zip\"")
			c.Set("Content-Type", "application/zip")

			dl := downloader.NewDownloader()

			c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
				zw := zip.NewWriter(w)
				defer zw.Close()

				for _, item := range itemsToZip {
					if item.File.TelegramMsgID == 0 {
						continue
					}

					res, err := api.MessagesGetMessages(ctx, []tg.InputMessageClass{
						&tg.InputMessageID{ID: item.File.TelegramMsgID},
					})
					if err != nil {
						continue
					}

					var doc *tg.Document
					if messages, ok := res.(*tg.MessagesMessages); ok && len(messages.Messages) > 0 {
						if msg, ok := messages.Messages[0].(*tg.Message); ok {
							if media, ok := msg.Media.(*tg.MessageMediaDocument); ok {
								if d, ok := media.Document.(*tg.Document); ok {
									doc = d
								}
							}
						}
					} else if messagesSlice, ok := res.(*tg.MessagesMessagesSlice); ok && len(messagesSlice.Messages) > 0 {
						if msg, ok := messagesSlice.Messages[0].(*tg.Message); ok {
							if media, ok := msg.Media.(*tg.MessageMediaDocument); ok {
								if d, ok := media.Document.(*tg.Document); ok {
									doc = d
								}
							}
						}
					}

					if doc == nil {
						continue
					}

					fWriter, err := zw.Create(item.ZipPath)
					if err != nil {
						continue
					}

					_, _ = dl.Download(api, doc.AsInputDocumentFileLocation("")).Stream(ctx, fWriter)
				}
				w.Flush()
			})

			return nil
		})
		apiGroup.Get("/download/:id", func(c *fiber.Ctx) error {
			id := c.Params("id")
			var fileRecord File
			if err := DB.First(&fileRecord, id).Error; err != nil {
				return c.Status(404).SendString("File tidak ditemukan di database")
			}

			if fileRecord.TelegramMsgID == 0 {
				return c.Status(400).SendString("File ini tidak memiliki referensi di Telegram")
			}

			res, err := api.MessagesGetMessages(ctx, []tg.InputMessageClass{
				&tg.InputMessageID{ID: fileRecord.TelegramMsgID},
			})
			if err != nil {
				return c.Status(500).SendString("Gagal menghubungi Telegram")
			}

			var doc *tg.Document
			if messages, ok := res.(*tg.MessagesMessages); ok && len(messages.Messages) > 0 {
				if msg, ok := messages.Messages[0].(*tg.Message); ok {
					if media, ok := msg.Media.(*tg.MessageMediaDocument); ok {
						if d, ok := media.Document.(*tg.Document); ok {
							doc = d
						}
					}
				}
			} else if messagesSlice, ok := res.(*tg.MessagesMessagesSlice); ok && len(messagesSlice.Messages) > 0 {
				if msg, ok := messagesSlice.Messages[0].(*tg.Message); ok {
					if media, ok := msg.Media.(*tg.MessageMediaDocument); ok {
						if d, ok := media.Document.(*tg.Document); ok {
							doc = d
						}
					}
				}
			}

			if doc == nil {
				return c.Status(404).SendString("Dokumen fisik tidak ditemukan di Telegram")
			}

			c.Set("Content-Disposition", "attachment; filename=\""+fileRecord.Name+"\"")
			c.Set("Content-Type", "application/octet-stream")
			c.Set("Content-Length", strconv.FormatInt(doc.Size, 10))

			dl := downloader.NewDownloader()
			c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
				_, err := dl.Download(api, doc.AsInputDocumentFileLocation("")).Stream(ctx, w)
				if err != nil {
					log.Printf("Error streaming file: %v\n", err)
				}
				w.Flush()
			})

			return nil
		})

		apiGroup.Get("/stream/:id", func(c *fiber.Ctx) error {
			id := c.Params("id")
			var fileRecord File
			if err := DB.First(&fileRecord, id).Error; err != nil {
				return c.Status(404).SendString("File tidak ditemukan")
			}
			if fileRecord.TelegramMsgID == 0 {
				return c.Status(400).SendString("File ini tidak memiliki referensi di Telegram")
			}

			res, err := api.MessagesGetMessages(ctx, []tg.InputMessageClass{
				&tg.InputMessageID{ID: fileRecord.TelegramMsgID},
			})
			if err != nil {
				return c.Status(500).SendString("Gagal menghubungi Telegram")
			}

			var doc *tg.Document
			if messages, ok := res.(*tg.MessagesMessages); ok && len(messages.Messages) > 0 {
				if msg, ok := messages.Messages[0].(*tg.Message); ok {
					if media, ok := msg.Media.(*tg.MessageMediaDocument); ok {
						if d, ok := media.Document.(*tg.Document); ok {
							doc = d
						}
					}
				}
			} else if messagesSlice, ok := res.(*tg.MessagesMessagesSlice); ok && len(messagesSlice.Messages) > 0 {
				if msg, ok := messagesSlice.Messages[0].(*tg.Message); ok {
					if media, ok := msg.Media.(*tg.MessageMediaDocument); ok {
						if d, ok := media.Document.(*tg.Document); ok {
							doc = d
						}
					}
				}
			}
			if doc == nil {
				return c.Status(404).SendString("Dokumen fisik tidak ditemukan")
			}

			ext := strings.ToLower(filepath.Ext(fileRecord.Name))
			mimeType := "application/octet-stream"
			switch ext {
			case ".jpg", ".jpeg":
				mimeType = "image/jpeg"
			case ".png":
				mimeType = "image/png"
			case ".gif":
				mimeType = "image/gif"
			case ".webp":
				mimeType = "image/webp"
			case ".mp4":
				mimeType = "video/mp4"
			case ".webm":
				mimeType = "video/webm"
			case ".pdf":
				mimeType = "application/pdf"
			}

			c.Set("Content-Disposition", "inline; filename=\""+fileRecord.Name+"\"")
			c.Set("Content-Type", mimeType)
			c.Set("Content-Length", strconv.FormatInt(doc.Size, 10))

			dl := downloader.NewDownloader()
			c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
				_, _ = dl.Download(api, doc.AsInputDocumentFileLocation("")).Stream(ctx, w)
				w.Flush()
			})
			return nil
		})

		apiGroup.Post("/file/:id/share", func(c *fiber.Ctx) error {
			id := c.Params("id")
			var file File
			if err := DB.First(&file, id).Error; err != nil {
				return c.Status(404).JSON(fiber.Map{"error": "File tidak ditemukan"})
			}

			if file.ShareHash != nil && *file.ShareHash != "" {
				if err := DB.Model(&file).Updates(map[string]interface{}{
					"share_hash":       gorm.Expr("NULL"),
					"share_expires_at": gorm.Expr("NULL"),
				}).Error; err != nil {
					return c.Status(500).JSON(fiber.Map{"error": "Gagal menghapus tautan"})
				}
				file.ShareHash = nil
			} else {
				newHash := fmt.Sprintf("%x", time.Now().UnixNano())
				expires := time.Now().Add(24 * time.Hour)

				if err := DB.Model(&file).Updates(map[string]interface{}{
					"share_hash":       newHash,
					"share_expires_at": expires,
				}).Error; err != nil {
					return c.Status(500).JSON(fiber.Map{"error": "Gagal membuat tautan baru"})
				}
				file.ShareHash = &newHash
			}

			return c.JSON(fiber.Map{
				"status":     "success",
				"share_hash": file.ShareHash,
			})
		})

		apiGroup.Post("/upload", func(c *fiber.Ctx) error {
			fileHeader, err := c.FormFile("file")
			if err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "File tidak ditemukan."})
			}

			fileStream, _ := fileHeader.Open()
			defer fileStream.Close()

			uploadCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
			defer cancel()

			uploadResult, err := up.Upload(uploadCtx, uploader.NewUpload(fileHeader.Filename, fileStream, fileHeader.Size))
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Gagal upload stream ke Telegram Server"})
			}

			msgUpdate, err := sender.Self().Media(uploadCtx, message.UploadedDocument(uploadResult).Filename(fileHeader.Filename))

			var tgMsgID int
			switch u := msgUpdate.(type) {
			case *tg.UpdateShortSentMessage:
				tgMsgID = u.ID
			case *tg.Updates:
				for _, upd := range u.Updates {
					if msgUpd, ok := upd.(*tg.UpdateNewMessage); ok {
						if msg, ok := msgUpd.Message.(*tg.Message); ok {
							tgMsgID = msg.ID
						}
					}
				}
			case *tg.UpdatesCombined:
				for _, upd := range u.Updates {
					if msgUpd, ok := upd.(*tg.UpdateNewMessage); ok {
						if msg, ok := msgUpd.Message.(*tg.Message); ok {
							tgMsgID = msg.ID
						}
					}
				}
			}

			log.Printf("📌 [DEBUG] File '%s' tersimpan dengan Telegram Message ID: %d\n", fileHeader.Filename, tgMsgID)

			newFile := File{
				Name:          fileHeader.Filename,
				Size:          fileHeader.Size,
				Type:          getFileType(fileHeader.Filename),
				TelegramMsgID: tgMsgID,
			}
			DB.Create(&newFile)

			return c.JSON(fiber.Map{"status": "success"})
		})

		apiGroup.Get("/files", func(c *fiber.Ctx) error {
			DB.Model(&File{}).Where("share_expires_at <= ?", time.Now()).Updates(map[string]interface{}{
				"share_hash":       gorm.Expr("NULL"),
				"share_expires_at": gorm.Expr("NULL"),
			})
			var files []File
			DB.Order("created_at desc").Find(&files)
			return c.JSON(fiber.Map{"status": "success", "data": files})
		})

		apiGroup.Get("/trash", func(c *fiber.Ctx) error {
			var files []File
			DB.Unscoped().Where("deleted_at IS NOT NULL").Order("deleted_at desc").Find(&files)
			return c.JSON(fiber.Map{"status": "success", "data": files})
		})

		apiGroup.Delete("/file/:id", func(c *fiber.Ctx) error {
			id := c.Params("id")
			DB.Delete(&File{}, id)
			return c.JSON(fiber.Map{"status": "success", "message": "File dipindah ke tempat sampah"})
		})

		apiGroup.Post("/file/:id/restore", func(c *fiber.Ctx) error {
			id := c.Params("id")
			DB.Unscoped().Model(&File{}).Where("id = ?", id).Update("deleted_at", nil)
			return c.JSON(fiber.Map{"status": "success", "message": "File dikembalikan"})
		})

		apiGroup.Delete("/file/:id/permanent", func(c *fiber.Ctx) error {
			id := c.Params("id")
			var file File

			if err := DB.Unscoped().First(&file, id).Error; err != nil {
				return c.Status(404).JSON(fiber.Map{"error": "File tidak ditemukan"})
			}
			if file.TelegramMsgID != 0 {
				affected, err := api.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
					ID:     []int{file.TelegramMsgID},
					Revoke: true,
				})
				if err != nil {
					log.Printf("⚠️ Gagal hapus di Telegram (MsgID: %d): %v\n", file.TelegramMsgID, err)
				} else {
					log.Printf("🗑️ [TELEGRAM DELETED] Pesan ID %d terhapus dari Saved Messages (Affected: %d)\n", file.TelegramMsgID, affected.PtsCount)
				}
			} else {
				log.Printf("⚠️ TelegramMsgID bernilai 0, file tidak memiliki referensi pesan di Telegram.")
			}

			DB.Unscoped().Delete(&file)

			return c.JSON(fiber.Map{"status": "success", "message": "File dihancurkan secara permanen"})
		})

		apiGroup.Put("/profile", func(c *fiber.Ctx) error {
			var req struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			}
			if err := c.BodyParser(&req); err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "Format salah"})
			}

			var user User
			if err := DB.First(&user).Error; err != nil {
				return c.Status(404).JSON(fiber.Map{"error": "User tidak ditemukan"})
			}

			user.Name = req.Name
			user.Email = req.Email
			DB.Save(&user)

			return c.JSON(fiber.Map{"status": "success", "user": fiber.Map{"name": user.Name, "email": user.Email}})
		})

		apiGroup.Put("/password", func(c *fiber.Ctx) error {
			var req struct {
				OldPassword string `json:"old_password"`
				NewPassword string `json:"new_password"`
			}
			if err := c.BodyParser(&req); err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "Format salah"})
			}

			var user User
			if err := DB.First(&user).Error; err != nil {
				return c.Status(404).JSON(fiber.Map{"error": "User tidak ditemukan"})
			}

			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "Password lama salah!"})
			}

			hash, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), 14)
			user.Password = string(hash)
			DB.Save(&user)

			return c.JSON(fiber.Map{"status": "success"})
		})

		app.Get("/s/:hash", func(c *fiber.Ctx) error {
			return c.SendFile("./public/share.html")
		})

		app.Get("/api/public/info/:hash", func(c *fiber.Ctx) error {
			hash := c.Params("hash")
			var fileRecord File
			if err := DB.Where("share_hash = ? AND share_expires_at > ?", hash, time.Now()).First(&fileRecord).Error; err != nil {
				return c.Status(404).JSON(fiber.Map{"error": "File tidak ditemukan atau akses dicabut"})
			}
			return c.JSON(fiber.Map{
				"status": "success",
				"data": fiber.Map{
					"name": fileRecord.Name,
					"size": fileRecord.Size,
					"type": fileRecord.Type,
				},
			})
		})

		app.Get("/api/public/download/:hash", func(c *fiber.Ctx) error {
			hash := c.Params("hash")
			var fileRecord File
			if err := DB.Where("share_hash = ? AND share_expires_at > ?", hash, time.Now()).First(&fileRecord).Error; err != nil {
				return c.Status(404).SendString("Link tidak valid atau kadaluarsa.")
			}

			res, err := api.MessagesGetMessages(ctx, []tg.InputMessageClass{&tg.InputMessageID{ID: fileRecord.TelegramMsgID}})
			if err != nil {
				return c.Status(500).SendString("Gagal menghubungi Telegram")
			}

			var doc *tg.Document
			if messages, ok := res.(*tg.MessagesMessages); ok && len(messages.Messages) > 0 {
				if msg, ok := messages.Messages[0].(*tg.Message); ok {
					if media, ok := msg.Media.(*tg.MessageMediaDocument); ok {
						if d, ok := media.Document.(*tg.Document); ok {
							doc = d
						}
					}
				}
			} else if messagesSlice, ok := res.(*tg.MessagesMessagesSlice); ok && len(messagesSlice.Messages) > 0 {
				if msg, ok := messagesSlice.Messages[0].(*tg.Message); ok {
					if media, ok := msg.Media.(*tg.MessageMediaDocument); ok {
						if d, ok := media.Document.(*tg.Document); ok {
							doc = d
						}
					}
				}
			}
			if doc == nil {
				return c.Status(404).SendString("Dokumen fisik tidak ditemukan")
			}

			c.Set("Content-Disposition", "attachment; filename=\""+fileRecord.Name+"\"")
			c.Set("Content-Type", "application/octet-stream")
			c.Set("Content-Length", strconv.FormatInt(doc.Size, 10))

			dl := downloader.NewDownloader()
			c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
				_, _ = dl.Download(api, doc.AsInputDocumentFileLocation("")).Stream(ctx, w)
				w.Flush()
			})
			return nil
		})

		log.Printf("🛡️ Engine IGO CLOUD Standby di %s\n", allowedOrigin)
		return app.Listen(":" + port)
	})

	if err != nil {
		log.Fatal("❌ Mesin Mati:", err)
	}
}
