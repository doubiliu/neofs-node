package searchsvc

import (
	"context"

	objectSDK "github.com/nspcc-dev/neofs-api-go/pkg/object"
	"github.com/nspcc-dev/neofs-node/pkg/core/object"
	"github.com/nspcc-dev/neofs-node/pkg/local_object_storage/bucket"
	"github.com/nspcc-dev/neofs-node/pkg/local_object_storage/localstore"
	"github.com/nspcc-dev/neofs-node/pkg/services/object/search/query"
	"github.com/pkg/errors"
)

type localStream struct {
	query query.Query

	storage *localstore.Storage
}

type searchQueryFilter struct {
	localstore.FilterPipeline

	query query.Query

	ch chan<- []*objectSDK.ID
}

func (s *localStream) stream(ctx context.Context, ch chan<- []*objectSDK.ID) error {
	idList := make([]*objectSDK.ID, 0)

	filter := &searchQueryFilter{
		query: s.query,
		ch:    ch,
	}

	if err := s.storage.Iterate(filter, func(meta *localstore.ObjectMeta) bool {
		select {
		case <-ctx.Done():
			return true
		default:
			idList = append(idList, meta.Head().GetID())

			return false
		}
	}); err != nil && !errors.Is(errors.Cause(err), bucket.ErrIteratingAborted) {
		return errors.Wrapf(err, "(%T) could not iterate over local storage", s)
	}

	ch <- idList

	return nil
}

func (f *searchQueryFilter) Pass(ctx context.Context, meta *localstore.ObjectMeta) *localstore.FilterResult {
loop:
	for obj := meta.Head(); obj.GetID() != nil; obj = object.NewFromSDK(obj.GetParent()) {
		if !f.query.Match(obj) {
			continue
		}

		select {
		case <-ctx.Done():
			break loop
		case f.ch <- []*objectSDK.ID{obj.GetID()}:
		}
	}

	return localstore.ResultPass()
}
