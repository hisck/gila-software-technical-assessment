package main

import (
	"ecommerce/internal/platform"
	"ecommerce/internal/search"
)

func main() {
	platform.Run("search", search.Schema, search.Setup)
}
