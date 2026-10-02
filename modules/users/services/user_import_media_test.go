package services

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	coreModels "josex/web/modules/core/models"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/services/storage/storagetest"
	importModels "josex/web/modules/import/models"
	userInterfaces "josex/web/modules/users/interfaces"
	userModels "josex/web/modules/users/models"
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

// refusingUsers answers every insert the way the create SP answers a taken email.
type refusingUsers struct {
	userInterfaces.UserService
}

func (refusingUsers) InsertUser(userModels.CreateUserDto) (*coreModels.User, error) {
	return nil, &pgconn.PgError{Message: "user.create.email.already-exists"}
}

// TestProcessRow_PictureOfAUserNotCreatedIsDeleted - the picture goes up before the user row; a row
// that ends skipped leaves no image behind (INFRA-007 step 6).
func TestProcessRow_PictureOfAUserNotCreatedIsDeleted(t *testing.T) {
	media := &recordingMedia{MediaService: coreServices.NewMediaService(storagetest.Media(t))}
	descriptor := &usersImportDescriptor{userService: refusingUsers{}, mediaService: media}
	ctx := importModels.ImportContext{Images: importModels.ImportImages(map[string][]byte{"ana.png": []byte("ana-bytes")})}

	result := descriptor.ProcessRow(ctx, 2, completeRow())

	if result.Status != importModels.RowStatusSkipped {
		t.Fatalf("expected the duplicate to be skipped, got %s %v", result.Status, result.Errors)
	}
	if len(media.saved) != 1 {
		t.Fatalf("expected the picture to have been stored once, got %v", media.saved)
	}
	res, err := http.Get(media.saved[0])
	if err != nil {
		t.Fatalf("GET %s: %v", media.saved[0], err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("expected the unused picture gone, got %d", res.StatusCode)
	}
}
