package products

type Product struct {
	ID          string
	Name        string
	Description string
	ImageURL    string

	Price float64

	Processor string
	RAM       string
	Drive     string
	Display   string
}

var Products = map[string]Product{}

type ProductMobile struct {
	ID1          string
	Name1        string
	Description1 string
	ImageURL1    string

	Price1 float64

	Display1 string
	Camera   string
	Drive1   string
	Color    string
}

var ProductsMobile = map[string]ProductMobile{}
