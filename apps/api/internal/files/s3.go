package files

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

const s3Service = "s3"

type S3Config struct {
	Endpoint       string
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	SessionToken   string
	AllowInsecure  bool
	RequestTimeout time.Duration
	TempDirectory  string
	HTTPClient     *http.Client
	Now            func() time.Time
}

// S3Store implements the small BlobStore surface against path-style,
// SigV4-compatible object storage. Uploads spool to bounded temporary storage
// so hashing never buffers the object in application memory.
type S3Store struct {
	endpoint      *url.URL
	region        string
	bucket        string
	accessKey     string
	secretKey     string
	sessionToken  string
	tempDirectory string
	client        *http.Client
	now           func() time.Time
}

func NewS3Store(cfg S3Config) (*S3Store, error) {
	endpoint, err := url.Parse(strings.TrimSpace(cfg.Endpoint))
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "https" && endpoint.Scheme != "http") {
		return nil, errors.New("S3 endpoint must be an absolute HTTP(S) URL")
	}
	if endpoint.Scheme != "https" && !cfg.AllowInsecure {
		return nil, errors.New("insecure S3 endpoint requires explicit opt-in")
	}
	if endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.User != nil {
		return nil, errors.New("S3 endpoint must not contain credentials, query, or fragment")
	}
	if strings.TrimSpace(cfg.Region) == "" || !validS3Name(cfg.Region) {
		return nil, errors.New("valid S3 region is required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" || !validS3Name(cfg.Bucket) {
		return nil, errors.New("valid S3 bucket is required")
	}
	if strings.TrimSpace(cfg.AccessKey) == "" || cfg.SecretKey == "" {
		return nil, errors.New("S3 access and secret keys are required")
	}
	client := cfg.HTTPClient
	if client == nil {
		timeout := cfg.RequestTimeout
		if timeout <= 0 {
			timeout = 60 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &S3Store{
		endpoint: endpoint, region: cfg.Region, bucket: cfg.Bucket,
		accessKey: cfg.AccessKey, secretKey: cfg.SecretKey, sessionToken: cfg.SessionToken,
		tempDirectory: cfg.TempDirectory, client: client, now: now,
	}, nil
}

func (store *S3Store) Put(
	ctx context.Context,
	key string,
	declaredMediaType string,
	reader io.Reader,
	maxBytes int64,
) (StoredBlob, error) {
	if !validStorageKey(key) || reader == nil {
		return StoredBlob{}, ErrInvalidStorageKey
	}
	if maxBytes < 1 {
		return StoredBlob{}, errors.New("positive object size limit is required")
	}
	temporary, err := os.CreateTemp(store.tempDirectory, ".crm-s3-upload-*")
	if err != nil {
		return StoredBlob{}, fmt.Errorf("create bounded S3 upload spool: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryName)
	}()

	buffered := bufio.NewReaderSize(reader, 32<<10)
	sniff, peekErr := buffered.Peek(512)
	if peekErr != nil && !errors.Is(peekErr, io.EOF) && !errors.Is(peekErr, bufio.ErrBufferFull) {
		return StoredBlob{}, fmt.Errorf("inspect upload: %w", peekErr)
	}
	mediaType, err := validateMediaType(declaredMediaType, sniff)
	if err != nil {
		return StoredBlob{}, err
	}
	hasher := sha256.New()
	written, err := copyWithContext(ctx, io.MultiWriter(temporary, hasher), buffered, maxBytes+1)
	if err != nil {
		return StoredBlob{}, fmt.Errorf("spool S3 upload: %w", err)
	}
	if written > maxBytes {
		return StoredBlob{}, ErrObjectTooLarge
	}
	if _, err := temporary.Seek(0, io.SeekStart); err != nil {
		return StoredBlob{}, fmt.Errorf("rewind S3 upload: %w", err)
	}
	payloadHash := hex.EncodeToString(hasher.Sum(nil))
	request, err := store.request(ctx, http.MethodPut, key, payloadHash, temporary)
	if err != nil {
		return StoredBlob{}, err
	}
	request.ContentLength = written
	request.Header.Set("Content-Type", mediaType)
	response, err := store.client.Do(request)
	if err != nil {
		return StoredBlob{}, fmt.Errorf("upload S3 object: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusNoContent {
		return StoredBlob{}, s3ResponseError("upload", response)
	}
	var checksum [sha256.Size]byte
	copy(checksum[:], hasher.Sum(nil))
	return StoredBlob{SizeBytes: written, ChecksumSHA256: checksum, MediaType: mediaType}, nil
}

func (store *S3Store) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	request, err := store.request(ctx, http.MethodGet, key, emptySHA256(), nil)
	if err != nil {
		return nil, err
	}
	response, err := store.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download S3 object: %w", err)
	}
	if response.StatusCode == http.StatusNotFound {
		_ = response.Body.Close()
		return nil, os.ErrNotExist
	}
	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		return nil, s3ResponseError("download", response)
	}
	return response.Body, nil
}

func (store *S3Store) Delete(ctx context.Context, key string) error {
	request, err := store.request(ctx, http.MethodDelete, key, emptySHA256(), nil)
	if err != nil {
		return err
	}
	response, err := store.client.Do(request)
	if err != nil {
		return fmt.Errorf("delete S3 object: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusNoContent || response.StatusCode == http.StatusOK {
		return nil
	}
	return s3ResponseError("delete", response)
}

func (store *S3Store) request(ctx context.Context, method, key, payloadHash string, body io.Reader) (*http.Request, error) {
	if !validStorageKey(key) {
		return nil, ErrInvalidStorageKey
	}
	target := *store.endpoint
	target.Path = strings.TrimRight(target.Path, "/") + "/" + store.bucket + "/" + key
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create S3 request: %w", err)
	}
	now := store.now().UTC()
	request.Header.Set("X-Amz-Date", now.Format("20060102T150405Z"))
	request.Header.Set("X-Amz-Content-Sha256", payloadHash)
	if store.sessionToken != "" {
		request.Header.Set("X-Amz-Security-Token", store.sessionToken)
	}
	store.sign(request, payloadHash, now)
	return request, nil
}

func (store *S3Store) sign(request *http.Request, payloadHash string, now time.Time) {
	headers := map[string]string{
		"host":                 request.URL.Host,
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           request.Header.Get("X-Amz-Date"),
	}
	if store.sessionToken != "" {
		headers["x-amz-security-token"] = store.sessionToken
	}
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	var canonicalHeaders strings.Builder
	for _, name := range names {
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(strings.Join(strings.Fields(headers[name]), " "))
		canonicalHeaders.WriteByte('\n')
	}
	signedHeaders := strings.Join(names, ";")
	canonicalRequest := strings.Join([]string{
		request.Method, request.URL.EscapedPath(), request.URL.Query().Encode(),
		canonicalHeaders.String(), signedHeaders, payloadHash,
	}, "\n")
	date := now.Format("20060102")
	scope := strings.Join([]string{date, store.region, s3Service, "aws4_request"}, "/")
	stringToSign := "AWS4-HMAC-SHA256\n" + request.Header.Get("X-Amz-Date") + "\n" + scope + "\n" + sha256Hex([]byte(canonicalRequest))
	dateKey := hmacSHA256([]byte("AWS4"+store.secretKey), date)
	regionKey := hmacSHA256(dateKey, store.region)
	serviceKey := hmacSHA256(regionKey, s3Service)
	signingKey := hmacSHA256(serviceKey, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	request.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+store.accessKey+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
}

func validS3Name(value string) bool {
	if len(value) < 1 || len(value) > 63 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' && character != '.' {
			return false
		}
	}
	return value[0] != '-' && value[len(value)-1] != '-'
}

func s3ResponseError(operation string, response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
	return fmt.Errorf("S3 %s failed with status %d: %s", operation, response.StatusCode, strings.TrimSpace(string(body)))
}

func emptySHA256() string { return sha256Hex(nil) }

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
