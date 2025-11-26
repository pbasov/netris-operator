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

	"github.com/netrisai/netriswebapi/v2/types/serverclustertemplate"
)

// ServerClusterTemplateStorage caches ServerClusterTemplate objects.
type ServerClusterTemplateStorage struct {
	sync.Mutex
	Templates []*serverclustertemplate.ServerClusterTemplate
}

// NewServerClusterTemplateStorage creates new storage.
func NewServerClusterTemplateStorage() *ServerClusterTemplateStorage {
	return &ServerClusterTemplateStorage{}
}

// GetAll returns all cached templates.
func (s *ServerClusterTemplateStorage) GetAll() []*serverclustertemplate.ServerClusterTemplate {
	s.Lock()
	defer s.Unlock()
	return s.Templates
}

// FindByName returns template by name.
func (s *ServerClusterTemplateStorage) FindByName(name string) (*serverclustertemplate.ServerClusterTemplate, bool) {
	s.Lock()
	defer s.Unlock()
	item, ok := s.findByName(name)
	if !ok {
		_ = s.download()
		return s.findByName(name)
	}
	return item, ok
}

func (s *ServerClusterTemplateStorage) findByName(name string) (*serverclustertemplate.ServerClusterTemplate, bool) {
	for _, t := range s.Templates {
		if t.Name == name {
			return t, true
		}
	}
	return nil, false
}

// FindByID returns template by ID.
func (s *ServerClusterTemplateStorage) FindByID(id int) (*serverclustertemplate.ServerClusterTemplate, bool) {
	s.Lock()
	defer s.Unlock()
	item, ok := s.findByID(id)
	if !ok {
		_ = s.download()
		return s.findByID(id)
	}
	return item, ok
}

func (s *ServerClusterTemplateStorage) findByID(id int) (*serverclustertemplate.ServerClusterTemplate, bool) {
	for _, t := range s.Templates {
		if t.ID == id {
			return t, true
		}
	}
	return nil, false
}

func (s *ServerClusterTemplateStorage) storeAll(items []*serverclustertemplate.ServerClusterTemplate) {
	s.Templates = items
}

func (s *ServerClusterTemplateStorage) download() error {
	items, err := Cred.ServerClusterTemplate().Get()
	if err != nil {
		return err
	}
	s.storeAll(items)
	return nil
}

// Download refreshes cached templates.
func (s *ServerClusterTemplateStorage) Download() error {
	s.Lock()
	defer s.Unlock()
	return s.download()
}
