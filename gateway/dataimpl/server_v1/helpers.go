package server_v1

import (
	"time"

	"github.com/couchbase/gocb/v2"
	"github.com/couchbase/stellar-nebula/genproto/kv_v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func casFromGocb(cas gocb.Cas) uint64 {
	return uint64(cas)
}

func casToGocb(cas *uint64) gocb.Cas {
	return gocb.Cas(*cas)
}

func timeFromGo(when time.Time) *timestamppb.Timestamp {
	// This is a workaround for a bug in Go where its Zero return values are not
	// actually matched with IsZero().
	// TODO(brett19): Remove this workaround when gocbcore is fixed.
	if when.Equal(time.Unix(0, 0)) {
		return nil
	}

	if when.IsZero() {
		return nil
	}
	return timestamppb.New(when)
}

func timeToGo(ts *timestamppb.Timestamp) time.Time {
	return ts.AsTime()
}

func durationFromGo(dura time.Duration) *durationpb.Duration {
	return durationpb.New(dura)
}

func durationToGo(d *durationpb.Duration) time.Duration {
	return d.AsDuration()
}

func tokenFromGocb(token *gocb.MutationToken) *kv_v1.MutationToken {
	if token == nil {
		return nil
	}

	return &kv_v1.MutationToken{
		BucketName:  token.BucketName(),
		VbucketId:   uint32(token.PartitionID()),
		VbucketUuid: token.PartitionUUID(),
		SeqNo:       token.SequenceNumber(),
	}
}

func durabilityLevelToGocb(dl kv_v1.DurabilityLevel) (gocb.DurabilityLevel, *status.Status) {
	switch dl {
	case kv_v1.DurabilityLevel_MAJORITY:
		return gocb.DurabilityLevelMajority, nil
	case kv_v1.DurabilityLevel_MAJORITY_AND_PERSIST_TO_ACTIVE:
		return gocb.DurabilityLevelMajorityAndPersistOnMaster, nil
	case kv_v1.DurabilityLevel_PERSIST_TO_MAJORITY:
		return gocb.DurabilityLevelPersistToMajority, nil
	}

	// TODO(brett19): We should attach the field reference information here indicating
	// what specific field the user incorrectly specified.
	return gocb.DurabilityLevel(0), status.New(codes.InvalidArgument, "invalid durability level specified")
}
