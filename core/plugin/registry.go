package plugin

import "fmt"

// Registry holds exactly one instance of each plugin type.
// It is the ONLY place where plugins are wired into the system —
// no other package (pipeline, cmd) may import plugin implementations directly.
// This keeps the dependency graph clean: pipeline depends on interfaces,
// not on concrete Tavily/BM25/Blocklist types.
//
// V0.2 adds multi-provider search, this struct holds multiple SearchPlugins.
type Registry struct {
	searchPlugins []SearchPlugin
	filter        FilterPlugin
	rank          RankPlugin
}

// NewRegistry constructs a Registry after verifying that every plugin slot is
// filled. Returning an error early is better than a nil-pointer panic deep
// inside the pipeline.
func NewRegistry(searchPlugins []SearchPlugin, filter FilterPlugin, rank RankPlugin) (*Registry, error) {
	if len(searchPlugins) == 0 {
		return nil, fmt.Errorf("plugin registry: at least one search plugin must be provided")
	}
	if filter == nil {
		return nil, fmt.Errorf("plugin registry: filter plugin must not be nil")
	}
	if rank == nil {
		return nil, fmt.Errorf("plugin registry: rank plugin must not be nil")
	}
	return &Registry{
		searchPlugins: searchPlugins,
		filter:        filter,
		rank:          rank,
	}, nil
}

// SearchPlugins returns the registered SearchPlugins.
func (r *Registry) SearchPlugins() []SearchPlugin { return r.searchPlugins }

// Filter returns the registered FilterPlugin.
func (r *Registry) Filter() FilterPlugin { return r.filter }

// Rank returns the registered RankPlugin.
func (r *Registry) Rank() RankPlugin { return r.rank }
