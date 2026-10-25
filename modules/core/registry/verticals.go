package registry

import (
	"slices"
	"sort"
)

// Vertical is a business type the onboarding offers (TENANCY-006): picking one turns on Modules, offers
// Suggested with a switch each, switches on Capabilities and, in the sign-up, seeds Starter categories.
// Names come from lang/ (`vertical.<code>.name`, `vertical.group.<group>`); Name is the English fallback.
type Vertical struct {
	Code  string `json:"code"`
	Group string `json:"group"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	// Defining is the module the business cannot run without (D1): until it is installed the type is
	// not offered, so nobody picks "Farmacia" and gets a generic shop.
	Defining     string   `json:"-"`
	Modules      []string `json:"modules"`
	Suggested    []string `json:"suggested"`
	Capabilities []string `json:"capabilities"`
	// Starter are category codes (`category.starter.<code>`) seeded on sign-up when inventory is on (D2).
	Starter []string `json:"-"`
	// Setup reorders the checklist's module steps for this type (TENANCY-008); empty keeps the modules' order.
	Setup []string `json:"-"`
}

// Capability is a function inside a module that a business type switches on (D3): the module that
// implements it registers it and hides what it does not use while it is off.
type Capability struct {
	Code   string `json:"code"`
	Module string `json:"module"`
	Name   string `json:"name"`
}

// CapabilitiesSetting is the tenant setting holding the capabilities switched on. A tenant that never
// stored it has every compiled capability on (D3): only a stored list restricts.
const CapabilitiesSetting = "capabilities"

// The groups, in the order the onboarding shows them.
const (
	GroupCommerce  = "commerce"
	GroupFood      = "food"
	GroupHealth    = "health"
	GroupServices  = "services"
	GroupTransport = "transport"
)

// Module codes a business type may name before the module exists; D1 hides the type until it does.
var (
	retailModules    = []string{"pos", "inventory", "sales", "users"}
	posModules       = []string{"pos", "sales", "users"}
	bookingModules   = []string{"booking", "pos", "sales", "users"}
	transportModules = []string{"tracking", "users"}
)

// verticals are declared here, in code: 0 tables, 0 queries. The matrix of Onb-matriz, in Onb-negocio's order.
var verticals = []Vertical{
	{Code: "grocery", Group: GroupCommerce, Name: "Grocery and minimarket", Icon: "cart", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"units", "weight"}, Starter: []string{"groceries", "beverages", "dairy", "cleaning", "snacks"}},
	{Code: "clothing", Group: GroupCommerce, Name: "Clothing and boutique", Icon: "shopping-bag", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"variants", "layaway"}, Starter: []string{"women", "men", "kids", "accessories"}},
	{Code: "shoes", Group: GroupCommerce, Name: "Shoe store", Icon: "tag", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"variants"}, Starter: []string{"women", "men", "kids", "sports"}},
	{Code: "toys", Group: GroupCommerce, Name: "Toy store", Icon: "games", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"variants", "layaway"}, Starter: []string{"babies", "board_games", "dolls", "outdoor"}},
	{Code: "fabrics", Group: GroupCommerce, Name: "Fabrics and haberdashery", Icon: "ruler", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"measures", "variants"}, Starter: []string{"fabrics", "haberdashery", "yarns"}},
	{Code: "hardware", Group: GroupCommerce, Name: "Hardware store", Icon: "toolbox", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"measures", "units", "quotes"}, Starter: []string{"tools", "electrical", "plumbing", "paint", "fasteners"}},
	{Code: "stationery", Group: GroupCommerce, Name: "Bookshop and stationery", Icon: "book", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"units", "layaway", "work_orders"}, Starter: []string{"school_supplies", "office", "books", "printing"}},
	{Code: "pet_store", Group: GroupCommerce, Name: "Pet shop", Icon: "animal-dog", Modules: retailModules, Suggested: []string{"shop", "booking"}, Capabilities: []string{"weight", "lots", "variants"}, Starter: []string{"pet_food", "accessories", "hygiene", "medicines"}},
	{Code: "electronics", Group: GroupCommerce, Name: "Phones and electronics", Icon: "phone", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"serials", "work_orders", "quotes"}, Starter: []string{"phones", "accessories", "audio", "repairs"}},
	{Code: "auto_parts", Group: GroupCommerce, Name: "Auto parts", Icon: "vehicle-car", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"units", "quotes", "serials"}, Starter: []string{"engine", "brakes", "electrical", "lubricants"}},
	{Code: "cosmetics", Group: GroupCommerce, Name: "Cosmetics and perfumery", Icon: "sparkle", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"variants", "lots"}, Starter: []string{"makeup", "skin_care", "fragrances", "hair"}},
	{Code: "liquor", Group: GroupCommerce, Name: "Liquor store", Icon: "drink-wine", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"units", "restricted"}, Starter: []string{"beer", "wine", "spirits", "soft_drinks"}},
	{Code: "florist", Group: GroupCommerce, Name: "Florist", Icon: "leaf-three", Modules: []string{"pos", "inventory", "shop", "sales", "users"}, Capabilities: []string{"layaway", "modifiers"}, Starter: []string{"bouquets", "arrangements", "plants", "gifts"}},

	{Code: "cafe", Group: GroupFood, Name: "Café", Icon: "drink-coffee", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"modifiers", "units"}, Starter: []string{"hot_drinks", "cold_drinks", "pastries", "sandwiches"}},
	{Code: "bakery", Group: GroupFood, Name: "Bakery and patisserie", Icon: "food-cake", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"units", "weight", "layaway"}, Starter: []string{"bread", "cakes", "pastries", "beverages"}},
	{Code: "restaurant", Group: GroupFood, Name: "Restaurant", Icon: "food", Defining: "restaurant", Modules: []string{"restaurant", "pos", "inventory", "sales", "users"}, Suggested: []string{"shop"}, Capabilities: []string{"modifiers", "tables"}, Starter: []string{"starters", "mains", "desserts", "beverages"}},
	{Code: "fast_food", Group: GroupFood, Name: "Rotisserie and fast food", Icon: "food-pizza", Modules: []string{"pos", "inventory", "shop", "sales", "users"}, Capabilities: []string{"modifiers", "units"}, Starter: []string{"combos", "chicken", "sides", "beverages"}},
	{Code: "ice_cream", Group: GroupFood, Name: "Ice cream parlour", Icon: "drink-to-go", Modules: posModules, Suggested: []string{"shop", "inventory"}, Capabilities: []string{"modifiers", "weight"}, Starter: []string{"ice_cream", "desserts", "beverages"}},
	{Code: "butcher", Group: GroupFood, Name: "Butcher", Icon: "scales", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"weight", "lots"}, Starter: []string{"beef", "pork", "chicken", "sausages"}},
	{Code: "greengrocer", Group: GroupFood, Name: "Greengrocer", Icon: "leaf-one", Modules: retailModules, Suggested: []string{"shop"}, Capabilities: []string{"weight"}, Starter: []string{"fruits", "vegetables", "greens"}},

	{Code: "pharmacy", Group: GroupHealth, Name: "Pharmacy", Icon: "pill", Defining: "pharmacy", Modules: []string{"pharmacy", "pos", "inventory", "sales", "users"}, Suggested: []string{"shop"}, Capabilities: []string{"lots", "restricted", "units"}, Starter: []string{"medicines", "personal_care", "baby", "vitamins"}},
	{Code: "optician", Group: GroupHealth, Name: "Optician", Icon: "glasses", Modules: []string{"pos", "inventory", "booking", "sales", "users"}, Capabilities: []string{"restricted", "work_orders", "appointments"}, Starter: []string{"frames", "lenses", "sunglasses", "contact_lenses"}},
	{Code: "veterinary", Group: GroupHealth, Name: "Veterinary clinic", Icon: "animal-cat", Modules: []string{"pos", "booking", "inventory", "sales", "users"}, Capabilities: []string{"appointments", "lots", "restricted"}, Starter: []string{"medicines", "pet_food", "accessories", "vaccines"}},

	{Code: "barber", Group: GroupServices, Name: "Hair salon and barber", Icon: "cut", Defining: "booking", Modules: bookingModules, Suggested: []string{"inventory"}, Capabilities: []string{"appointments", "memberships"}, Starter: []string{"hair", "beard"}},
	{Code: "spa", Group: GroupServices, Name: "Spa and beauty", Icon: "heart", Defining: "booking", Modules: bookingModules, Suggested: []string{"inventory"}, Capabilities: []string{"appointments", "memberships"}, Starter: []string{"skin_care", "makeup"}},
	{Code: "gym", Group: GroupServices, Name: "Gym", Icon: "dumbbell", Defining: "booking", Modules: bookingModules, Capabilities: []string{"memberships", "appointments"}, Starter: []string{"supplements", "beverages"}},
	{Code: "laundry", Group: GroupServices, Name: "Laundry", Icon: "drop", Modules: posModules, Suggested: []string{"shop"}, Capabilities: []string{"work_orders", "weight"}},
	{Code: "mechanic", Group: GroupServices, Name: "Car repair shop", Icon: "wrench", Modules: retailModules, Suggested: []string{"booking"}, Capabilities: []string{"work_orders", "quotes", "serials"}, Starter: []string{"spare_parts", "lubricants", "tyres"}},

	{Code: "school_transport", Group: GroupTransport, Name: "School transport", Icon: "backpack", Defining: "tracking", Modules: transportModules},
	{Code: "staff_transport", Group: GroupTransport, Name: "Staff transport", Icon: "vehicle-bus", Defining: "tracking", Modules: transportModules},
	// Deliveries are orders taken to a door (SHOP-004), not people moved: Pedidos with couriers, no tracking.
	{Code: "delivery", Group: GroupTransport, Name: "Delivery and courier", Icon: "vehicle-truck-profile", Defining: "shop", Modules: []string{"shop", "sales", "users"}, Capabilities: []string{"couriers"}},
	{Code: "intercity_bus", Group: GroupTransport, Name: "Intercity buses", Icon: "ticket-horizontal", Defining: "ticketing", Modules: []string{"ticketing", "users"}},
	{Code: "radio_taxi", Group: GroupTransport, Name: "Radio taxi", Icon: "vehicle-cab", Defining: "dispatch", Modules: []string{"dispatch", "users"}},
}

// StarterNames are the English names of the starter category codes, the fallback when lang/ has none.
var StarterNames = map[string]string{
	"groceries":       "Groceries",
	"beverages":       "Beverages",
	"dairy":           "Dairy",
	"cleaning":        "Cleaning",
	"snacks":          "Snacks",
	"women":           "Women",
	"men":             "Men",
	"kids":            "Kids",
	"accessories":     "Accessories",
	"sports":          "Sports",
	"babies":          "Babies",
	"board_games":     "Board games",
	"dolls":           "Dolls and figures",
	"outdoor":         "Outdoor",
	"fabrics":         "Fabrics",
	"haberdashery":    "Haberdashery",
	"yarns":           "Threads and yarns",
	"tools":           "Tools",
	"electrical":      "Electrical",
	"plumbing":        "Plumbing",
	"paint":           "Paint",
	"fasteners":       "Screws and fasteners",
	"school_supplies": "School supplies",
	"office":          "Office",
	"books":           "Books",
	"printing":        "Printing",
	"pet_food":        "Pet food",
	"hygiene":         "Hygiene",
	"medicines":       "Medicines",
	"phones":          "Phones",
	"audio":           "Audio",
	"repairs":         "Repairs",
	"engine":          "Engine",
	"brakes":          "Brakes",
	"lubricants":      "Lubricants",
	"makeup":          "Makeup",
	"skin_care":       "Skin care",
	"fragrances":      "Fragrances",
	"hair":            "Hair care",
	"beer":            "Beer",
	"wine":            "Wine",
	"spirits":         "Spirits",
	"soft_drinks":     "Soft drinks",
	"bouquets":        "Bouquets",
	"arrangements":    "Arrangements",
	"plants":          "Plants",
	"gifts":           "Gifts",
	"hot_drinks":      "Hot drinks",
	"cold_drinks":     "Cold drinks",
	"pastries":        "Pastries",
	"sandwiches":      "Sandwiches",
	"bread":           "Bread",
	"cakes":           "Cakes",
	"starters":        "Starters",
	"mains":           "Main courses",
	"desserts":        "Desserts",
	"combos":          "Combos",
	"chicken":         "Chicken",
	"sides":           "Sides",
	"ice_cream":       "Ice cream",
	"beef":            "Beef",
	"pork":            "Pork",
	"sausages":        "Sausages",
	"fruits":          "Fruits",
	"vegetables":      "Vegetables",
	"greens":          "Greens and herbs",
	"personal_care":   "Personal care",
	"baby":            "Baby",
	"vitamins":        "Vitamins",
	"frames":          "Frames",
	"lenses":          "Lenses",
	"sunglasses":      "Sunglasses",
	"contact_lenses":  "Contact lenses",
	"vaccines":        "Vaccines",
	"beard":           "Beard care",
	"supplements":     "Supplements",
	"spare_parts":     "Spare parts",
	"tyres":           "Tyres",
}

var capabilities = map[string]Capability{}

// RegisterCapability adds or replaces a capability, and offers it in the capabilities setting. Called
// from the owning module's init().
func RegisterCapability(c Capability) {
	mu.Lock()
	defer mu.Unlock()
	capabilities[c.Code] = c
	codes := make([]string, 0, len(capabilities))
	for code := range capabilities {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	settings[CapabilitiesSetting] = SettingSpec{Key: CapabilitiesSetting, Module: "tenancy", Type: SettingSet, Default: codes, Options: codes}
}

// Capabilities answers every registered capability, sorted by code.
func Capabilities() []Capability {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Capability, 0, len(capabilities))
	for _, c := range capabilities {
		out = append(out, c)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Code < out[b].Code })
	return out
}

// CapabilityOn reports whether a tenant's stored settings switch capability on: every one is on until a
// list is stored (D3).
func CapabilityOn(stored map[string]any, code string) bool {
	spec, ok := SettingFor(CapabilitiesSetting)
	if !ok {
		return false
	}
	on, _ := spec.Value(stored).([]string)
	return slices.Contains(on, code)
}

// OfferedVerticals answers the business types this deployment can serve (D1), each narrowed to the
// modules installed and the capabilities those modules register. A type is offered when one of its
// business modules is installed and, if it declares one, its Defining module is.
func OfferedVerticals(installed func(code string) bool) []Vertical {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Vertical, 0, len(verticals))
	for _, v := range verticals {
		if v.Defining != "" && !installed(v.Defining) {
			continue
		}
		modules := keep(v.Modules, installed)
		if !slices.ContainsFunc(modules, func(code string) bool { return code != "users" }) {
			continue
		}
		v.Modules = modules
		v.Suggested = keep(v.Suggested, installed)
		v.Capabilities = keep(v.Capabilities, func(code string) bool {
			c, ok := capabilities[code]
			return ok && installed(c.Module)
		})
		out = append(out, v)
	}
	return out
}

// Primary is the module a business opens on (APP-015 D2): its Defining one, else its first business module.
func (v Vertical) Primary() string {
	if v.Defining != "" {
		return v.Defining
	}
	for _, code := range v.Modules {
		if code != "users" {
			return code
		}
	}
	return ""
}

func keep(codes []string, ok func(string) bool) []string {
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		if ok(code) {
			out = append(out, code)
		}
	}
	return out
}
