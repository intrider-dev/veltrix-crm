package app

import (
	"io"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/files"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/config"
)

func newAttachmentService(cfg config.Config, application *Application) (*files.Service, io.Closer, error) {
	var (
		store  files.BlobStore
		closer io.Closer
	)
	if cfg.StorageBackend == "s3" {
		s3Store, err := files.NewS3Store(files.S3Config{
			Endpoint: cfg.S3Endpoint, Region: cfg.S3Region, Bucket: cfg.S3Bucket,
			AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, SessionToken: cfg.S3SessionToken,
			AllowInsecure: cfg.S3AllowInsecure, RequestTimeout: 60 * time.Second,
		})
		if err != nil {
			return nil, nil, err
		}
		store = s3Store
	} else {
		localStore, err := files.NewLocalStore(cfg.UploadDir)
		if err != nil {
			return nil, nil, err
		}
		store = localStore
		closer = localStore
	}
	service, err := files.NewService(store, files.UnavailableScanner{}, application, cfg.MaxUploadBytes)
	if err != nil {
		if closer != nil {
			_ = closer.Close()
		}
		return nil, nil, err
	}
	return service, closer, nil
}
