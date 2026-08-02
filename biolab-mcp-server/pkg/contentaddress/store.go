package contentaddress

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

type Store struct {
	root string
}

func NewStore(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func (s *Store) Put(data []byte) (string, error) {
	hash := sha256.Sum256(data)
	key := hex.EncodeToString(hash[:])
	
	path := s.pathForKey(key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	
	return key, nil
}

func (s *Store) PutReader(r io.Reader) (string, error) {
	hash := sha256.New()
	tee := io.TeeReader(r, hash)
	data, err := io.ReadAll(tee)
	if err != nil {
		return "", err
	}
	key := hex.EncodeToString(hash.Sum(nil))
	
	path := s.pathForKey(key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	
	return key, nil
}

func (s *Store) Get(key string) ([]byte, error) {
	path := s.pathForKey(key)
	return os.ReadFile(path)
}

func (s *Store) GetReader(key string) (io.ReadCloser, error) {
	path := s.pathForKey(key)
	return os.Open(path)
}

func (s *Store) Exists(key string) bool {
	path := s.pathForKey(key)
	_, err := os.Stat(path)
	return err == nil
}

func (s *Store) Delete(key string) error {
	path := s.pathForKey(key)
	return os.Remove(path)
}

func (s *Store) pathForKey(key string) string {
	// Shard by first 2 chars for filesystem performance
	if len(key) < 2 {
		return filepath.Join(s.root, key)
	}
	return filepath.Join(s.root, key[:2], key[2:])
}