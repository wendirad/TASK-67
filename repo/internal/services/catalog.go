package services

import (
	"campusrec/internal/models"
	"campusrec/internal/repository"
)

type CatalogService struct {
	catalogRepo *repository.CatalogRepository
}

func NewCatalogService(catalogRepo *repository.CatalogRepository) *CatalogService {
	return &CatalogService{catalogRepo: catalogRepo}
}

func (s *CatalogService) Query(q repository.CatalogQuery) ([]models.CatalogItem, int, error) {
	return s.catalogRepo.Query(q)
}
