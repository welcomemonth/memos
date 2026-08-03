package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/serpapi/serpapi-golang"
)

func main() {
	_ = godotenv.Load(".env")
	// replace with your SerpApi key
	setting := serpapi.NewSerpApiClientSetting(os.Getenv("SERPAPI_KEY"))
	setting.Engine = "google"
	// initialize the client
	client := serpapi.NewClient(setting)
	parameter := map[string]string{
		"q":             "特朗普多大了",
		"location":      "Beijing, China",
		"google_domain": "google.com.hk",
		"gl":            "cn",
		"hl":            "zh-CN",
		"num":           "5",
	}
	results, err := client.Search(parameter)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(results)
}
