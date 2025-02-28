module github.com/next-exp/decoder_go

go 1.22.0

require (
	github.com/go-sql-driver/mysql v1.8.1
	github.com/jmbenlloch/go-hdf5 v0.0.0-20250122105311-a0f7f2bfa567
	github.com/jmoiron/sqlx v1.4.0
	github.com/magefile/mage v1.15.0
	golang.org/x/exp v0.0.0-20250106191152-7588d65b2ba8
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	gonum.org/v1/hdf5 v0.0.0-20210714002203-8c5d23bc6946 // indirect
)

//replace github.com/jmbenlloch/go-hdf5 => /home/jmbenlloch/go/hdf5
//replace github.com/next-exp/decoder_go/pkg => /home/jmbenlloch/next/decoder_go/pkg
