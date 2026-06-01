module github.com/byte-v-forge/wa-app

go 1.26

toolchain go1.26.3

require (
	github.com/byte-v-forge/common-lib v0.0.0
	github.com/jackc/pgx/v5 v5.9.2
	github.com/nyaruka/phonenumbers v1.7.5
	go.mozilla.org/pkcs7 v0.9.0
	go.step.sm/crypto v0.81.1
	golang.org/x/crypto v0.52.0
	golang.org/x/sync v0.20.0
	google.golang.org/grpc v1.81.1
	google.golang.org/protobuf v1.36.11
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/nats-io/nats.go v1.52.0 // indirect
	github.com/nats-io/nkeys v0.4.15 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/redis/go-redis/v9 v9.19.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260511170946-3700d4141b60 // indirect
)

replace github.com/byte-v-forge/common-lib => ../common-lib
