package issueopsrecord

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"agent-harness/internal/adapter/outbound/sqlstore"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

var bucket = fmt.Sprintf("issueops_v%d", issueopscontract.IssueOpsSchemaVersion)

type Store struct {
	Scope    string
	Observer Observer
}

type Mutation func(
	issueopscontract.IssueOpsRecord,
) (issueopscontract.IssueOpsRecord, bool, error)

type RelatedMutation func(
	issueopscontract.IssueOpsRecord,
	[]byte,
	bool,
) ([]byte, bool, error)

func Bucket() string {
	return bucket
}

func (Store) Read(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopscontract.IssueOpsRecord, error) {
	if err := ctx.Err(); err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	id, err := NormalizeID(id)
	if err != nil {
		return issueopscontract.IssueOpsRecord{OK: false}, err
	}
	data, found, err := sqlstore.GetExisting(stateRoot, bucket, id)
	if err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	if !found {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, fmt.Errorf(
			"issueops record %s: %w",
			id,
			fs.ErrNotExist,
		)
	}
	return Decode(id, data)
}

func (Store) ListIDs(ctx context.Context, stateRoot string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids, err := sqlstore.ListExisting(stateRoot, bucket)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	return ids, err
}

func (store Store) Update(
	ctx context.Context,
	stateRoot string,
	id string,
	mutate Mutation,
) (issueopscontract.IssueOpsRecord, error) {
	id, database, err := open(ctx, stateRoot, id)
	if err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	result := issueopscontract.IssueOpsRecord{OK: false, ID: id}
	err = database.WithSpan(store.observe(ctx, "update"), func(spanContext context.Context) error {
		record, err := readLocked(spanContext, database, id)
		if err != nil {
			return err
		}
		var changed bool
		result, changed, err = mutate(record)
		if err != nil || !changed {
			return err
		}
		data, err := Encode(result)
		if err != nil {
			return err
		}
		return database.Put(bucket, id, data)
	})
	if err != nil {
		result.OK = false
		return result, err
	}
	result.OK = true
	return result, nil
}

func (store Store) UpdateRelated(
	ctx context.Context,
	stateRoot string,
	id string,
	relatedBucket string,
	mutate RelatedMutation,
) (issueopscontract.IssueOpsRecord, error) {
	id, database, err := open(ctx, stateRoot, id)
	if err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	result := issueopscontract.IssueOpsRecord{OK: false, ID: id}
	err = database.WithSpan(store.observe(ctx, "related_update"), func(spanContext context.Context) error {
		record, err := readLocked(spanContext, database, id)
		if err != nil {
			return err
		}
		data, found, err := database.Get(relatedBucket, id)
		if err != nil {
			return err
		}
		data, remove, err := mutate(record, data, found)
		if err != nil {
			return err
		}
		if remove {
			err = database.Delete(relatedBucket, id)
		} else {
			err = database.Put(relatedBucket, id, data)
		}
		if err == nil {
			result = record
		}
		return err
	})
	if err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	result.OK = true
	return result, nil
}

func (Store) ReadRelated(
	ctx context.Context,
	stateRoot string,
	id string,
	relatedBucket string,
) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	id, err := NormalizeID(id)
	if err != nil {
		return nil, false, err
	}
	return sqlstore.GetExisting(stateRoot, relatedBucket, id)
}

func (Store) Delete(
	ctx context.Context,
	stateRoot string,
	id string,
	relatedBuckets ...string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id, err := NormalizeID(id)
	if err != nil {
		return err
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	mutations := make([]port.RecordMutation, 0, len(relatedBuckets)+1)
	for _, relatedBucket := range relatedBuckets {
		mutations = append(mutations, port.RecordMutation{
			Bucket: relatedBucket,
			ID:     id,
			Delete: true,
		})
	}
	mutations = append(mutations, port.RecordMutation{Bucket: bucket, ID: id, Delete: true})
	return database.Apply(ctx, mutations)
}

func open(ctx context.Context, stateRoot, id string) (string, *sqlstore.DB, error) {
	if err := ctx.Err(); err != nil {
		return id, nil, err
	}
	id, err := NormalizeID(id)
	if err != nil {
		return id, nil, err
	}
	database, err := sqlstore.Open(stateRoot)
	return id, database, err
}

func readLocked(
	ctx context.Context,
	database *sqlstore.DB,
	id string,
) (issueopscontract.IssueOpsRecord, error) {
	if err := ctx.Err(); err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	data, found, err := database.Get(bucket, id)
	if err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	if !found {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, fmt.Errorf(
			"issueops record %s: %w",
			id,
			fs.ErrNotExist,
		)
	}
	record, err := Decode(id, data)
	if err != nil {
		return record, err
	}
	if record.CleanupAbandonFailure != nil &&
		record.CleanupAbandonFailure.Step == "applying" {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, fmt.Errorf(
			"cleanup abandon apply is in progress",
		)
	}
	return record, nil
}
