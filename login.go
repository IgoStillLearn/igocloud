package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/joho/godotenv"
)

type termAuth struct {
	phone string
}

func (a termAuth) Phone(_ context.Context) (string, error) { return a.phone, nil }
func (a termAuth) Password(_ context.Context) (string, error) {
	fmt.Print("Masukkan Password 2FA (Tekan Enter jika tidak ada 2FA): ")
	p, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(p), nil
}
func (a termAuth) AcceptTermsOfService(_ context.Context, _ tg.HelpTermsOfService) error { return nil }
func (a termAuth) SignUp(_ context.Context) (auth.UserInfo, error)                       { return auth.UserInfo{}, nil }
func (a termAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	fmt.Print("Masukkan Kode OTP dari Telegram: ")
	code, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(code), nil
}

func mainLogin() {
	godotenv.Load()
	apiID, _ := strconv.Atoi(os.Getenv("TG_API_ID"))
	apiHash := os.Getenv("TG_API_HASH")
	phone := os.Getenv("TG_PHONE")

	ctx := context.Background()

	sessionStorage := &session.FileStorage{Path: "session.json"}

	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		SessionStorage: sessionStorage,
	})

	fmt.Println("Menghubungkan ke server Telegram...")

	err := client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if status.Authorized {
			fmt.Println("✅ Status: Sudah Login! File session.json aman.")
			return nil
		}

		fmt.Println("⏳ Status: Belum Login. Meminta kode OTP...")
		flow := auth.NewFlow(termAuth{phone: phone}, auth.SendCodeOptions{})
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return err
		}

		fmt.Println("LOGIN SUCCESS 'session.json'")
		return nil
	})

	if err != nil {
		fmt.Println("Terjadi error:", err)
	}
}
