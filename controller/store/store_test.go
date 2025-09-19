package store

import (
	"context"
	"sync"
	"testing"
)

func TestSQLiteStore_ConcurrencyAndPersist(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	n := &Network{ID: "n1", Name: "TestNet", Description: "desc"}
	if err := s.CreateNetwork(context.Background(), n); err != nil {
		t.Fatalf("create: %v", err)
	}

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				nn, err := s.GetNetwork(context.Background(), "n1")
				if err != nil || nn.ID != "n1" {
					t.Errorf("get: %v", err)
				}
				nn.Name = "TestNet" + string('A'+i)
				_ = s.UpdateNetwork(context.Background(), nn)
			}
		}(i)
	}
	wg.Wait()

	// Persist: close and reopen
	s.Close()
	s2, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	_, err = s2.GetNetwork(context.Background(), "n1")
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
}
