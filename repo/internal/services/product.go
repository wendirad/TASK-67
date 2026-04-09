package services

import (
	"campusrec/internal/models"
	"campusrec/internal/repository"
)

type ProductService struct {
	productRepo *repository.ProductRepository
}

func NewProductService(productRepo *repository.ProductRepository) *ProductService {
	return &ProductService{productRepo: productRepo}
}

func (s *ProductService) ListProducts(page, pageSize int, category, search, status string, minPrice, maxPrice *int, isShippable *bool) ([]models.Product, int, error) {
	return s.productRepo.List(page, pageSize, category, search, status, minPrice, maxPrice, isShippable)
}

func (s *ProductService) GetProduct(id string) (*models.Product, error) {
	return s.productRepo.FindByID(id)
}
