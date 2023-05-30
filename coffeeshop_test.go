package coffeeshop_test

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/qba73/coffeeshop"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

func newCoffeShopTestServer(store coffeeshop.Store, latency string, t *testing.T) *coffeeshop.Server {
	t.Helper()

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	addr := l.Addr().String()
	cs, err := coffeeshop.New(addr, store, coffeeshop.WithLatency(latency))
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		err := cs.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	// Cleanup is called after each test function.
	// We do not need to call `defer server close` in each test function.
	t.Cleanup(func() {
		err := cs.Shutdown(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	})
	return cs
}

func TestGetAll_ReturnsAllItemsFromStore(t *testing.T) {
	t.Parallel()

	memoryStore := coffeeshop.MemoryStore{
		Products: map[string]coffeeshop.Product{
			"1": {ID: "1", Name: "Coffee", Brand: "Segafredo"},
			"2": {ID: "2", Name: "Coffee", Brand: "illy"},
		},
	}

	want := []coffeeshop.Product{
		{ID: "1", Name: "Coffee", Brand: "Segafredo"},
		{ID: "2", Name: "Coffee", Brand: "illy"},
	}

	got := memoryStore.GetAll()

	if !cmp.Equal(want, got, cmpopts.SortSlices(func(i, j coffeeshop.Product) bool { return i.ID < j.ID })) {
		t.Error(cmp.Diff(want, got))
	}
}

func TestGetProduct_ReturnsSingleItemFromStore(t *testing.T) {
	t.Parallel()

	memoryStore := coffeeshop.MemoryStore{
		Products: inventory,
	}

	want := coffeeshop.Product{
		ID:       "2",
		Type:     "Coffee",
		Brand:    "Segafredo",
		Name:     "Caffé Crema Gustoso",
		Unit:     "gram",
		Quantity: "1000",
		Price:    "11.99",
		Properties: []coffeeshop.Property{
			{Name: "flavour", Value: "Acidic Robusta, Nuts, Aromatic Arabica, Medium roasted beans"},
			{Name: "property", Value: "1000 grams, Arabica/Robusta"},
			{Name: "intensity", Value: "Medium (6/10)"},
		},
	}

	got, err := memoryStore.GetProduct("2")
	if err != nil {
		t.Fatal(err)
	}

	if !cmp.Equal(want, got) {
		t.Error(cmp.Diff(want, got))
	}
}

func TestServer_Returns200OnValidGetProductsRequest(t *testing.T) {
	t.Parallel()

	store := &coffeeshop.MemoryStore{
		Products: inventory,
	}

	shop := newCoffeShopTestServer(store, "100ms", t)
	resp, err := http.Get(shop.URL + "products")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Error(resp.StatusCode)
	}
}

func TestServer_ReturnsAllProducts(t *testing.T) {
	t.Parallel()

	store := &coffeeshop.MemoryStore{
		Products: inventory,
	}

	shop := newCoffeShopTestServer(store, "100ms", t)
	resp, err := http.Get(shop.URL + "products")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want HTTP 200OK, got %d", resp.StatusCode)
	}

	var got []coffeeshop.Product
	err = json.NewDecoder(resp.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}

	want := maps.Values(inventory)
	if !cmp.Equal(want, got, cmpopts.SortSlices(func(i, j coffeeshop.Product) bool { return i.ID < j.ID })) {
		t.Error(cmp.Diff(want, got))
	}
}

func TestServer_Returns404OnNotExistingProduct(t *testing.T) {
	t.Parallel()

	store := &coffeeshop.MemoryStore{
		Products: inventory,
	}

	shop := newCoffeShopTestServer(store, "100ms", t)
	resp, err := http.Get(shop.URL + "products/20")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want HTTP 404, got %d", resp.StatusCode)
	}
}

func TestServer_ReturnsSingleProduct(t *testing.T) {
	t.Parallel()

	store := &coffeeshop.MemoryStore{
		Products: inventory,
	}

	shop := newCoffeShopTestServer(store, "100ms", t)
	resp, err := http.Get(shop.URL + "products/1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want HTTP 200OK, got %d", resp.StatusCode)
	}

	var got coffeeshop.Product
	err = json.NewDecoder(resp.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}

	// We need to make sure products in the inventory var are sorted for testing.
	px := maps.Values(inventory)
	slices.SortStableFunc(px, func(i, j coffeeshop.Product) bool { return i.ID < j.ID })

	// We called GET /products/1, so we need to pick first item from the sorted slice.
	want := px[0]

	if !cmp.Equal(want, got) {
		t.Error(cmp.Diff(want, got))
	}
}

func TestServer_ReturnsSingleProductAfterConfiguredDelay(t *testing.T) {
	t.Parallel()

	store := &coffeeshop.MemoryStore{
		Products: inventory,
	}

	shop := newCoffeShopTestServer(store, "2s", t)

	start := time.Now()
	resp, err := http.Get(shop.URL + "products/2")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatal(resp.StatusCode)
	}

	stop := time.Now()
	got := stop.Sub(start)
	want := 2 * time.Second
	margin := 100 * time.Millisecond

	if (want - got).Abs() > margin {
		t.Error(cmp.Diff(want-got, margin))
	}
}

var (
	inventory = coffeeshop.Products{
		"1": {
			ID:       "1",
			Type:     "Coffee",
			Brand:    "Segafredo",
			Name:     "Intermezzo",
			Unit:     "gram",
			Quantity: "1000",
			Price:    "7.99",
			Properties: []coffeeshop.Property{
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
			Properties: []coffeeshop.Property{
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
			Properties: []coffeeshop.Property{
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
			Properties: []coffeeshop.Property{
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
			Properties: []coffeeshop.Property{
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
			Properties: []coffeeshop.Property{
				{Name: "flavour", Value: "Aromatic Arabica, Medium roasted beans"},
				{Name: "property", Value: "250 gram, Arabica"},
				{Name: "intensity", Value: "Medium (6/10)"},
			},
		},
	}
)
