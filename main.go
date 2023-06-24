package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

type Route[C any] struct {
	Method  string
	Path    string
	Handler func(c C) error
}

func NewHTTPRoute[C any](method string, path string, handler func(c C) error) Route[C] {
	return Route[C]{
		Method:  method,
		Path:    path,
		Handler: handler,
	}
}

func NewHTTPServer(address string, port string, routes []Route[*fiber.Ctx]) error {
	server := fiber.New()
	server.Use(func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "http://127.0.0.1") // TODO: change to your domain.
		return c.Next()
	})

	for _, route := range routes {
		switch route.Method {
		case http.MethodGet:
			server.Get(route.Path, route.Handler)
		case http.MethodPost:
			server.Post(route.Path, route.Handler)
		case http.MethodPut:
			server.Put(route.Path, route.Handler)
		case http.MethodDelete:
			server.Delete(route.Path, route.Handler)
		case http.MethodPatch:
			server.Patch(route.Path, route.Handler)
		default:
			return fmt.Errorf("invalid method %s", route.Method)
		}
	}

	return server.Listen(address + ":" + port)
}

type Connection interface {
	List() ([]string, error)
}

type MemConnection struct{}

func (c *MemConnection) List() ([]string, error) {
	return []string{
		"SKU2006-001",
		"SKU2006-002",
		"SKU2006-003",
	}, nil
}

type Database interface {
	Connect() (Connection, error)
	Disconnect() error
}

// MemDatabase
type MemDatabase struct{}

func (db *MemDatabase) Connect() (Connection, error) {
	return &MemConnection{}, nil
}

func (db *MemDatabase) Disconnect() error {
	return nil
}

func NewMemDatabase() (Database, error) {
	return &MemDatabase{}, nil
}

/*
	You need to build this project from scratch. In summary:
	1. User purchased a service dog package on your shopify website.
	2. An SKU computing service webhook is triggered with the payload of the purchase.
	3. Based on the settings of the SKUs in the db, some specific data is retrieved from the webhook request payload.
	4. The retrieved data will be sent to your service dog backend service.
*/

type Servicer interface {
	Webhook(sku string) error
}

type Services struct {
	database Connection
}

func (s *Services) Webhook(sku string) error {
	skus, _ := s.database.List()

	for _, s := range skus {
		if s == sku {
			// Do something when a wanted sku is found.
			fmt.Println("Found wanted sku:", sku)
			return errors.New("Found wanted sku")
		}
	}

	return nil
}

func NewServices(connection Connection) (*Services, error) {
	return &Services{
		database: connection,
	}, nil
}

func main() {
	const Signature = ""

	database, err := NewMemDatabase()
	if err != nil {
		log.Fatal(err)
	}

	connection, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}

	services, err := NewServices(connection)
	if err != nil {
		log.Fatal(err)
	}

	routes := []Route[*fiber.Ctx]{
		NewHTTPRoute[*fiber.Ctx](http.MethodGet, "/ping", func(c *fiber.Ctx) error {
			return c.SendString("pong")
		}),
		NewHTTPRoute[*fiber.Ctx](http.MethodPost, "/webhook", func(c *fiber.Ctx) error {
			headers := c.GetReqHeaders()
			signature := headers["X-Shopify-Hmac-Sha256"]
			{
				h := hmac.New(sha256.New, []byte(Signature))
				h.Write(c.Request().Body())
				d := h.Sum(nil)
				m := base64.StdEncoding.EncodeToString(d)

				if !hmac.Equal([]byte(m), []byte(signature)) {
					return c.SendStatus(http.StatusUnauthorized)
				}
			}

			var body struct {
				LineItems []struct {
					Sku string `json:"sku"`
				} `json:"line_items"`
			}

			if err := c.BodyParser(&body); err != nil {
				return err
			}

			for _, lineItem := range body.LineItems {
				if err := services.Webhook(lineItem.Sku); err != nil {
					return err
				}
			}

			return c.SendStatus(http.StatusOK)
		}),
	}

	log.Fatal(NewHTTPServer("localhost", "8080", routes))
}
