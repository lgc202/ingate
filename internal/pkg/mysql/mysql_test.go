package mysql

import (
	"errors"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestParseDSNPreservesConnectionSettings(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
	}{
		{
			name: "default time and statement options",
			dsn:  "test@tcp(127.0.0.1:3307)/ingate?timeout=3s&readTimeout=4s&writeTimeout=5s&sql_mode=%27ANSI%27",
		},
		{
			name: "override conflicting time and statement options",
			dsn:  "test@tcp(127.0.0.1:3307)/ingate?timeout=3s&readTimeout=4s&writeTimeout=5s&sql_mode=%27ANSI%27&parseTime=false&loc=Asia%2FShanghai&multiStatements=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseDSN(tt.dsn)
			if err != nil {
				t.Fatalf("ParseDSN(%s) error = %v, want nil", tt.name, err)
			}

			if !config.ParseTime || config.Loc != time.UTC || config.MultiStatements {
				t.Errorf("ParseDSN(%s) parseTime/loc/multiStatements = %t/%v/%t, want true/UTC/false",
					tt.name, config.ParseTime, config.Loc, config.MultiStatements)
			}

			if config.User != "test" || config.Net != "tcp" || config.Addr != "127.0.0.1:3307" || config.DBName != "ingate" {
				t.Errorf("ParseDSN(%s) user/net/addr/database = %q/%q/%q/%q, want test/tcp/127.0.0.1:3307/ingate",
					tt.name, config.User, config.Net, config.Addr, config.DBName)
			}

			if config.Timeout != 3*time.Second || config.ReadTimeout != 4*time.Second || config.WriteTimeout != 5*time.Second {
				t.Errorf("ParseDSN(%s) dial/read/write timeout = %v/%v/%v, want 3s/4s/5s",
					tt.name, config.Timeout, config.ReadTimeout, config.WriteTimeout)
			}

			if diff := cmp.Diff(map[string]string{"sql_mode": "'ANSI'"}, config.Params); diff != "" {
				t.Errorf("ParseDSN(%s) parameters mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestParseDSNPreservesDriverOptions(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
	}{
		{name: "driver defaults"},
		{name: "unix socket", dsn: "test@unix(/tmp/ingate-test-mysql.sock)/ingate"},
		{
			name: "explicit driver options",
			dsn:  "test:test-only-password@tcp(127.0.0.1:3307)/ingate?charset=utf8mb4&collation=utf8mb4_unicode_ci&timeTruncate=2s&compress=true&clientFoundRows=true&interpolateParams=true&tls=false&connectionAttributes=service%3Aingate&maxAllowedPacket=16384&sql_mode=%27ANSI%27&parseTime=false&loc=Local&multiStatements=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := mysqldriver.ParseDSN(tt.dsn)
			if err != nil {
				t.Fatalf("driver ParseDSN(%s) error = %v, want nil", tt.name, err)
			}

			want.ParseTime = true
			want.Loc = time.UTC
			want.MultiStatements = false

			got, err := ParseDSN(tt.dsn)
			if err != nil {
				t.Fatalf("ParseDSN(%s) error = %v, want nil", tt.name, err)
			}

			// 比较公开配置；规范 DSN 另行覆盖 charset、压缩等驱动私有选项。
			options := []cmp.Option{
				cmpopts.IgnoreUnexported(mysqldriver.Config{}),
				cmpopts.IgnoreFields(mysqldriver.Config{}, "Logger"),
				cmp.Comparer(func(a, b *time.Location) bool { return a == b }),
			}
			if diff := cmp.Diff(want, got, options...); diff != "" {
				t.Errorf("ParseDSN(%s) driver options mismatch (-want +got):\n%s", tt.name, diff)
			}

			if gotDSN, wantDSN := got.FormatDSN(), want.FormatDSN(); gotDSN != wantDSN {
				t.Errorf("ParseDSN(%s).FormatDSN() = %q, want driver-normalized DSN %q", tt.name, gotDSN, wantDSN)
			}

			roundTrip, err := mysqldriver.ParseDSN(got.FormatDSN())
			if err != nil {
				t.Fatalf("driver ParseDSN(FormatDSN(%s)) error = %v, want nil", tt.name, err)
			}

			if diff := cmp.Diff(want, roundTrip, options...); diff != "" {
				t.Errorf("ParseDSN(%s) FormatDSN round trip mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestParseDSNRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
	}{
		{name: "missing separator", dsn: "invalid-dsn-without-database-separator"},
		{name: "unterminated address", dsn: "test@tcp(127.0.0.1:3306/ingate"},
		{name: "unsafe interpolation collation", dsn: "test@tcp(127.0.0.1:3306)/ingate?interpolateParams=true&collation=gbk_chinese_ci"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, wantCause := mysqldriver.ParseDSN(tt.dsn)
			if wantCause == nil {
				t.Fatalf("driver ParseDSN(%s) error = nil, want invalid fixture", tt.name)
			}

			config, err := ParseDSN(tt.dsn)
			if config != nil || !errors.Is(err, wantCause) {
				t.Errorf("ParseDSN(%s) = %v, %v, want nil config and driver cause %v", tt.name, config, err, wantCause)
			}

			if err == nil || !strings.HasPrefix(err.Error(), "parse MySQL DSN: ") {
				t.Errorf("ParseDSN(%s) error = %v, want original parse context", tt.name, err)
			}
		})
	}
}

func TestParseDSNReturnsIndependentDriverConfigs(t *testing.T) {
	const dsn = "test@tcp(127.0.0.1:3306)/ingate?sql_mode=%27ANSI%27"
	first, err := ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN(first) error = %v, want nil", err)
	}

	second, err := ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN(second) error = %v, want nil", err)
	}

	first.DBName = "changed"
	first.Params["sql_mode"] = "changed"
	if first == second || second.DBName != "ingate" || second.Params["sql_mode"] != "'ANSI'" {
		t.Errorf("ParseDSN(second) shares config=%t, database=%q, sql_mode=%q, want false/ingate/'ANSI'", first == second, second.DBName, second.Params["sql_mode"])
	}
}

func TestOpenRejectsInvalidDSN(t *testing.T) {
	const dsn = "invalid-dsn-without-database-separator"
	_, wantCause := mysqldriver.ParseDSN(dsn)
	if wantCause == nil {
		t.Fatal("ParseDSN(invalid DSN) error = nil, want driver parse failure")
	}

	db, err := Open(dsn)
	if !errors.Is(err, wantCause) {
		t.Errorf("Open(invalid DSN) error = %v, want driver cause %v", err, wantCause)
	}

	if db != nil {
		t.Error("Open(invalid DSN) database is non-nil, want nil")
	}
}
