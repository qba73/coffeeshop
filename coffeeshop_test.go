package coffeeshop_test

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/qba73/coffeeshop"
)

func newCoffeShopTestServer(store coffeeshop.Store, t *testing.T) *coffeeshop.Server {
	t.Helper()

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	addr := l.Addr().String()
	cs := coffeeshop.New(addr, store)

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

func TestServer_Returns200OnValidGetProductsRequest(t *testing.T) {
	t.Parallel()

	store := &coffeeshop.MemoryStore{
		Products: inventory,
	}

	shop := newCoffeShopTestServer(store, t)
	resp, err := http.Get(shop.URL + "products")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Error(resp.StatusCode)
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
