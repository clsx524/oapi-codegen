//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=types.cfg.yaml ../../petstore-expanded.yaml
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=server.cfg.yaml ../../petstore-expanded.yaml

package api

import (
	"context"
	"fmt"
	"sync"
)

type PetStore struct {
	Pets   map[int64]Pet
	NextId int64
	Lock   sync.Mutex
}

// Make sure we conform to StrictServerInterface

var _ StrictServerInterface = (*PetStore)(nil)

func NewPetStore() *PetStore {
	return &PetStore{
		Pets:   make(map[int64]Pet),
		NextId: 1000,
	}
}

// FindPets implements all the handlers in the ServerInterface
func (p *PetStore) FindPets(ctx context.Context, request FindPetsRequestObject) (FindPetsResponseObject, error) {
	p.Lock.Lock()
	defer p.Lock.Unlock()

	var result []Pet

	for _, pet := range p.Pets {
		if request.Params.Tags != nil {
			// If we have tags,  filter pets by tag
			for _, t := range *request.Params.Tags {
				if pet.Tag != nil && (*pet.Tag == t) {
					result = append(result, pet)
				}
			}
		} else {
			// Add all pets if we're not filtering
			result = append(result, pet)
		}

		if request.Params.Limit != nil {
			l := int(*request.Params.Limit)
			if len(result) >= l {
				// We're at the limit
				break
			}
		}
	}

	return FindPets200JSONResponse(result), nil
}

func (p *PetStore) AddPet(ctx context.Context, request AddPetRequestObject) (AddPetResponseObject, error) {
	// We now have a pet, let's add it to our "database".
	// We're always asynchronous, so lock unsafe operations below
	p.Lock.Lock()
	defer p.Lock.Unlock()

	// We handle pets, not NewPets, which have an additional ID field
	var pet Pet
	pet.Name = request.Body.Name
	pet.Tag = request.Body.Tag
	pet.ID = p.NextId
	p.NextId++

	// Insert into map
	p.Pets[pet.ID] = pet

	// Now, we have to return the NewPet
	return AddPet200JSONResponse(pet), nil
}

func (p *PetStore) FindPetByID(ctx context.Context, request FindPetByIDRequestObject) (FindPetByIDResponseObject, error) {
	p.Lock.Lock()
	defer p.Lock.Unlock()

	pet, found := p.Pets[request.Id]
	if !found {
		// TODO: Replace with proper FindPetByIDdefaultJSONResponse when strict server generation is fixed
		return FindPetByID200JSONResponse{}, fmt.Errorf("pet with ID %d not found", request.Id)
	}

	return FindPetByID200JSONResponse(pet), nil
}

func (p *PetStore) DeletePet(ctx context.Context, request DeletePetRequestObject) (DeletePetResponseObject, error) {
	p.Lock.Lock()
	defer p.Lock.Unlock()

	_, found := p.Pets[request.Id]
	if !found {
		// TODO: Replace with proper DeletePetdefaultJSONResponse when strict server generation is fixed
		return DeletePet204Response{}, fmt.Errorf("pet with ID %d not found", request.Id)
	}
	delete(p.Pets, request.Id)

	return DeletePet204Response{}, nil
}
