package coffeeshop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/exp/maps"
)

// Product represents a product in the inventory.
type Product struct {
	ID         string     `json:"id"`
	Type       string     `json:"type"`
	Brand      string     `json:"brand"`
	Name       string     `json:"name"`
	Unit       string     `json:"unit,omitempty"`
	Quantity   string     `json:"quantity,omitempty"`
	Price      string     `json:"price,omitempty"`
	Properties []Property `json:"properties,omitempty"`
}

// Property holds additional, dynamic information about
// the product.
type Property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Products map[string]Product

func (p Products) MarshalJSON() ([]byte, error) {
	type ProductsAlias Products
	pa := ProductsAlias(p)
	data, err := json.Marshal(pa)
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func (p *Products) UnmarshalJSON(data []byte) error {
	type ProductsAlias Products
	var pa ProductsAlias
	if err := json.Unmarshal(data, &pa); err != nil {
		return err
	}
	*p = Products(pa)
	return nil
}

// MemoryStore represents a storage for products
// in the CoffeeShop.
//
// Use memory store for testing and development.
// For production use a SQL or NoSQL database.
type MemoryStore struct {
	mx       sync.RWMutex
	Products Products
}

// GetAll returns all products in the store.
func (ms *MemoryStore) GetAll() []Product {
	ms.mx.RLock()
	defer ms.mx.RUnlock()
	return maps.Values(ms.Products)
}

func (ms *MemoryStore) GetProduct(id string) (Product, error) {
	ms.mx.RLock()
	defer ms.mx.RUnlock()
	p, ok := ms.Products[id]
	if !ok {
		return Product{}, errors.New("product not found")
	}
	return p, nil
}

func (ms *MemoryStore) GetCoffee() []Product {
	ms.mx.RLock()
	defer ms.mx.RUnlock()
	var coffee []Product
	for _, p := range maps.Values(ms.Products) {
		if strings.ToLower(p.Type) == "coffee" {
			coffee = append(coffee, p)
		}
	}
	return coffee
}

func (ms *MemoryStore) GetTea() []Product {
	ms.mx.RLock()
	defer ms.mx.RUnlock()
	var tea []Product
	for _, p := range maps.Values(ms.Products) {
		if strings.ToLower(p.Type) == "tea" {
			tea = append(tea, p)
		}
	}
	return tea
}

type Store interface {
	GetAll() []Product
	GetProduct(id string) (Product, error)
	GetCoffee() []Product
	GetTea() []Product
}

func latencyFromEnv(key, fallback string) (time.Duration, error) {
	if value, ok := os.LookupEnv(key); ok {
		d, err := time.ParseDuration(value)
		if err != nil {
			return 0, err
		}
		return d, nil
	}
	d, err := time.ParseDuration(fallback)
	if err != nil {
		return 0, err
	}
	return d, nil
}

type Server struct {
	HTTPServer *http.Server
	URL        string
	Latency    time.Duration
	Store      Store
}

type option func(s *Server) error

func WithLatency(l string) option {
	return func(s *Server) error {
		d, err := time.ParseDuration(l)
		if err != nil {
			return err
		}
		s.Latency = d
		return nil
	}
}

func New(addr string, store Store, options ...option) (*Server, error) {
	latency, err := latencyFromEnv("COFFEESHOP_LATENCY", "100m")
	if err != nil {
		return nil, err

	}

	srv := Server{
		HTTPServer: &http.Server{
			Addr:         addr,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		URL:     fmt.Sprintf("http://%s/", addr),
		Latency: latency,
		Store:   store,
	}

	for _, opt := range options {
		if err := opt(&srv); err != nil {
			return nil, err
		}
	}
	return &srv, nil
}

func Delay(d time.Duration) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(d)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func (cs *Server) ListenAndServe() error {
	mux := chi.NewRouter()
	mux.Use(
		middleware.Timeout(120*time.Second),
		middleware.SetHeader("Content-Type", "application/json; charset=utf-8"),
		Delay(cs.Latency),
	)
	mux.Get("/products", cs.GetProducts)
	mux.Get("/products/{productID}", cs.GetProduct)
	mux.Get("/products/tea", cs.GetTea)
	mux.Get("/products/coffee", cs.GetCoffee)
	cs.HTTPServer.Handler = mux
	return cs.HTTPServer.ListenAndServe()
}

func (cs *Server) Shutdown(ctx context.Context) error {
	return cs.HTTPServer.Shutdown(ctx)
}

func (cs *Server) GetProducts(w http.ResponseWriter, r *http.Request) {
	products := cs.Store.GetAll()
	data, err := json.MarshalIndent(products, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(data); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (cs *Server) GetProduct(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "productID")
	product, err := cs.Store.GetProduct(productID)
	if err != nil {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}
	data, err := json.MarshalIndent(product, "", "  ")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_, err = w.Write(data)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (cs *Server) GetCoffee(w http.ResponseWriter, r *http.Request) {
	products := cs.Store.GetCoffee()
	if len(products) == 0 {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}
	data, err := json.MarshalIndent(products, "", "  ")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_, err = w.Write(data)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (cs *Server) GetTea(w http.ResponseWriter, r *http.Request) {
	products := cs.Store.GetTea()
	if len(products) == 0 {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}
	data, err := json.MarshalIndent(products, "", "  ")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_, err = w.Write(data)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func Run() error {
	store := MemoryStore{
		Products: inventory,
	}
	addr := fmt.Sprintf(":%s", strconv.Itoa(8080))
	server, err := New(addr, &store, WithLatency("2s"))
	if err != nil {
		return err
	}
	return server.ListenAndServe()
}

var inventory = map[string]Product{
	"1": {
		ID:       "1",
		Type:     "Coffee",
		Brand:    "Segafredo",
		Name:     "Intermezzo",
		Unit:     "gram",
		Quantity: "1000",
		Price:    "7.99",
		Properties: []Property{
			{Name: "flavour", Value: "Acidic Robusta, Nuts, Aromatic Arabica, Caramel, Medium roasted beans"},
			{Name: "property", Value: "1000 grams, Arabica/Robusta"},
			{Name: "intensity", Value: ""},
		},
	},

	"2": {
		ID:       "2",
		Type:     "Coffee",
		Brand:    "Segafredo",
		Name:     "Caffé Crema Gustoso",
		Unit:     "gram",
		Quantity: "1000",
		Price:    "11.99",
		Properties: []Property{
			{Name: "flavour", Value: "Acidic Robusta, Nuts, Aromatic Arabica, Medium roasted beans"},
			{Name: "property", Value: "1000 grams, Arabica/Robusta"},
			{Name: "intensity", Value: "Medium (6/10)"},
		},
	},

	"3": {
		ID:       "3",
		Type:     "Coffee",
		Brand:    "Segafredo",
		Name:     "Selezione Espresso",
		Unit:     "gram",
		Quantity: "1000",
		Price:    "10.49",
		Properties: []Property{
			{Name: "flavour", Value: "Dark Chocolate, Acidic Robusta, Dark roasted beans, Aromatic Arabica"},
			{Name: "property", Value: "1000 grams, Arabica/Robusta"},
		},
	},

	"4": {
		ID:       "4",
		Type:     "Coffee",
		Brand:    "illy",
		Name:     "Intenso",
		Unit:     "gram",
		Quantity: "250",
		Price:    "7.99",
		Properties: []Property{
			{Name: "flavour", Value: "Fruit, Chocolate, Dark roasted beans, Bitterness"},
			{Name: "property", Value: "250 grams, Arabica"},
			{Name: "intensity", Value: "Very strong (9/10)"},
		},
	},

	"5": {
		ID:       "5",
		Type:     "Coffee",
		Brand:    "illy",
		Name:     "Guatemala",
		Unit:     "gram",
		Quantity: "250",
		Price:    "7.99",
		Properties: []Property{
			{Name: "flavour", Value: "Honey, Caramel, Sweetness"},
			{Name: "property", Value: "250 gram, Arabica"},
			{Name: "intensity", Value: "Medium (6/10)"},
		},
	},

	"6": {
		ID:       "6",
		Type:     "Coffee",
		Brand:    "Lavazza",
		Name:     "Espresso Barista Perfetto",
		Unit:     "gram",
		Quantity: "1000",
		Price:    "12.99",
		Properties: []Property{
			{Name: "flavour", Value: "Aromatic Arabica, Medium roasted beans"},
			{Name: "property", Value: "250 gram, Arabica"},
			{Name: "intensity", Value: "Medium (6/10)"},
		},
	},

	"7": {
		ID:       "7",
		Type:     "Tea",
		Brand:    "Caykur",
		Name:     "Green Tea",
		Unit:     "gram",
		Quantity: "150",
		Price:    "4.99",
	},

	"8": {
		ID:       "8",
		Type:     "Tea",
		Brand:    "Greeting Opine",
		Name:     "Jasmin Tea",
		Unit:     "gram",
		Quantity: "250",
		Price:    "7.49",
	},
}
