package main

import (
	"fmt"
	"log"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
	fmt.Println("=== Пример работы клиента с метриками через OpenTelemetry/Prometheus ===")
	client, err := httpclient.NewClient(
		httpclient.WithMetrics(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	// Выполняем тестовый запрос
	client.Get("https://httpbin.org/get")
	// Метрики теперь доступны только через Prometheus/OTel endpoint, а не через локальные методы.
}
