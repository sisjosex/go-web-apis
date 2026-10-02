package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/services/storage/storagetest"
	importModels "josex/web/modules/import/models"
	inventoryErrors "josex/web/modules/inventory/errors"
	inventoryInterfaces "josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"
)

// recordingMedia remembers every URL it stored, so a test can check what became of it.
type recordingMedia struct {
	coreServices.MediaService
	saved []string
}

func (m *recordingMedia) Save(ctx context.Context, category string, file coreServices.MediaFile) (string, error) {
	url, err := m.MediaService.Save(ctx, category, file)
	if err == nil {
		m.saved = append(m.saved, url)
	}
	return url, err
}

// refusingProducts fails every media row, the way a vanished product or a database error does.
type refusingProducts struct {
	inventoryInterfaces.ProductService
}

func (refusingProducts) AddProductMedia(context.Context, uuid.UUID, string, models.AddProductMediaDto) (*models.AddProductMediaResponse, error) {
	return nil, errors.New("media row refused")
}

// TestAttachProductImage_ImageWithoutARowIsDeleted - the image goes up before its media row; when the
// row is refused the image is deleted and the row still only warns (INFRA-007 step 6).
func TestAttachProductImage_ImageWithoutARowIsDeleted(t *testing.T) {
	media := &recordingMedia{MediaService: coreServices.NewMediaService(storagetest.Media(t))}
	descriptor := &productsImportDescriptor{productService: refusingProducts{}, mediaService: media}
	ctx := importModels.ImportContext{Images: importModels.ImportImages(map[string][]byte{"yerba.png": []byte("yerba-bytes")})}

	warnings := descriptor.attachProductImage(ctx, uuid.NewString(), map[string]string{"image": "yerba.png"})

	if len(warnings) != 1 || warnings[0] != inventoryErrors.ImportMediaFailed {
		t.Fatalf("expected the media warning, got %v", warnings)
	}
	if len(media.saved) != 1 {
		t.Fatalf("expected the image to have been stored once, got %v", media.saved)
	}
	res, err := http.Get(media.saved[0])
	if err != nil {
		t.Fatalf("GET %s: %v", media.saved[0], err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("expected the unused image gone, got %d", res.StatusCode)
	}
}
