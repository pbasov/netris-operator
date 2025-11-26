/*
Copyright 2021. Netris, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package netrisstorage

import (
	"sync"

	"github.com/netrisai/netriswebapi/v2/types/servercluster"
)

// ServerClusterStorage caches ServerCluster objects.
type ServerClusterStorage struct {
	sync.Mutex
	Clusters []*servercluster.ServerCluster
}

// NewServerClusterStorage creates new storage.
func NewServerClusterStorage() *ServerClusterStorage {
	return &ServerClusterStorage{}
}

// GetAll returns all cached clusters.
func (s *ServerClusterStorage) GetAll() []*servercluster.ServerCluster {
	s.Lock()
	defer s.Unlock()
	return s.Clusters
}

// FindByName returns cluster by name.
func (s *ServerClusterStorage) FindByName(name string) (*servercluster.ServerCluster, bool) {
	s.Lock()
	defer s.Unlock()
	item, ok := s.findByName(name)
	if !ok {
		_ = s.download()
		return s.findByName(name)
	}
	return item, ok
}

func (s *ServerClusterStorage) findByName(name string) (*servercluster.ServerCluster, bool) {
	for _, c := range s.Clusters {
		if c.Name == name {
			return c, true
		}
	}
	return nil, false
}

// FindByID returns cluster by ID.
func (s *ServerClusterStorage) FindByID(id int) (*servercluster.ServerCluster, bool) {
	s.Lock()
	defer s.Unlock()
	item, ok := s.findByID(id)
	if !ok {
		_ = s.download()
		return s.findByID(id)
	}
	return item, ok
}

func (s *ServerClusterStorage) findByID(id int) (*servercluster.ServerCluster, bool) {
	for _, c := range s.Clusters {
		if c.ID == id {
			return c, true
		}
	}
	return nil, false
}

func (s *ServerClusterStorage) storeAll(items []*servercluster.ServerCluster) {
	s.Clusters = items
}

func (s *ServerClusterStorage) download() error {
	items, err := Cred.ServerCluster().Get()
	if err != nil {
		return err
	}
	s.storeAll(items)
	return nil
}

// Download refreshes cached clusters.
func (s *ServerClusterStorage) Download() error {
	s.Lock()
	defer s.Unlock()
	return s.download()
}
