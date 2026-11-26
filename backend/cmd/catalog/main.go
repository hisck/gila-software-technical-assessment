package main

import (
	"ecommerce/internal/catalog"
	"ecommerce/internal/platform"
)

func main() {
	platform.Run("catalog", catalog.Schema, catalog.Setup)
}
