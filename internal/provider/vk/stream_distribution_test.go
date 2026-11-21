package vk

import (
	"context"
	"testing"

	"github.com/samosvalishe/free-turn-proxy/internal/provider"
	"github.com/samosvalishe/free-turn-proxy/internal/provider/multi"
	"github.com/samosvalishe/free-turn-proxy/internal/provider/vk/internal/vkauth"
)

type countingCredentialProvider struct {
	store   *vkauth.Store
	streams int
	groups  map[int]bool
}

func (p *countingCredentialProvider) GetCredentials(_ context.Context, id int) (provider.Credentials, error) {
	p.streams++
	p.groups[p.store.CacheID(id)] = true
	return provider.Credentials{}, nil
}
func (*countingCredentialProvider) IsAuthError(error) bool   { return false }
func (*countingCredentialProvider) HandleAuthError(int) bool { return false }
func (*countingCredentialProvider) ResetErrors(int)          {}
func (*countingCredentialProvider) DropCredentials(int)      {}
func (*countingCredentialProvider) BackoffUntilUnix() int64  { return 0 }
func (*countingCredentialProvider) Name() string             { return "counter" }

func TestPerAccountGroupingIsIndependentForEachLink(t *testing.T) {
	for _, test := range []struct{ links, perLink, perAccount int }{
		{1, 12, 12}, {2, 12, 12}, {3, 12, 4}, {2, 12, 5}, {3, 13, 5}, {2, 1, 1}, {2, 4, 12},
	} {
		providers := make([]provider.Provider, test.links)
		counters := make([]*countingCredentialProvider, test.links)
		for i := range counters {
			counters[i] = &countingCredentialProvider{store: vkauth.NewStore(test.perAccount), groups: map[int]bool{}}
			providers[i] = counters[i]
		}
		pool := multi.New(providers)
		for stream := 1; stream <= test.perLink*test.links; stream++ {
			if _, err := pool.GetCredentials(context.Background(), stream); err != nil {
				t.Fatal(err)
			}
		}
		wantGroups := (test.perLink + test.perAccount - 1) / test.perAccount
		for _, counter := range counters {
			if counter.streams != test.perLink || len(counter.groups) != wantGroups {
				t.Fatalf("%+v: got %d streams and %d credential groups per link, want %d and %d", test, counter.streams, len(counter.groups), test.perLink, wantGroups)
			}
		}
	}
}
