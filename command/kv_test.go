package command

import (
	"io"
	"strings"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/cli"
)

func testKVPutCommand(tb testing.TB) (*cli.MockUi, *KVPutCommand) {
	tb.Helper()

	ui := cli.NewMockUi()
	return ui, &KVPutCommand{
		BaseCommand: &BaseCommand{
			UI: ui,
		},
	}
}

func TestKVPutCommand(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		out  string
		code int
	}{
		{
			"not_enough_args",
			[]string{},
			"Not enough arguments",
			1,
		},
		{
			"empty_kvs",
			[]string{"secret/write/foo"},
			"Must supply data",
			1,
		},
		{
			"kvs_no_value",
			[]string{"secret/write/foo", "foo"},
			"Failed to parse K=V data",
			1,
		},
		{
			"single_value",
			[]string{"secret/write/foo", "foo=bar"},
			"Success!",
			0,
		},
		{
			"multi_value",
			[]string{"secret/write/foo", "foo=bar", "zip=zap"},
			"Success!",
			0,
		},
		{
			"v2_single_value",
			[]string{"kv/write/foo", "foo=bar"},
			"created_time",
			0,
		},
		{
			"v2_multi_value",
			[]string{"kv/write/foo", "foo=bar", "zip=zap"},
			"created_time",
			0,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, closer := testVaultServer(t)
			defer closer()

			if err := client.Sys().Mount("kv/", &api.MountInput{
				Type: "kv-v2",
			}); err != nil {
				t.Fatal(err)
			}

			ui, cmd := testKVPutCommand(t)
			cmd.client = client

			code := cmd.Run(tc.args)
			if code != tc.code {
				t.Errorf("expected %d to be %d", code, tc.code)
			}

			combined := ui.OutputWriter.String() + ui.ErrorWriter.String()
			if !strings.Contains(combined, tc.out) {
				t.Errorf("expected %q to contain %q", combined, tc.out)
			}
		})
	}

	t.Run("v2_cas", func(t *testing.T) {
		t.Parallel()

		client, closer := testVaultServer(t)
		defer closer()

		if err := client.Sys().Mount("kv/", &api.MountInput{
			Type: "kv-v2",
		}); err != nil {
			t.Fatal(err)
		}

		ui, cmd := testKVPutCommand(t)
		cmd.client = client

		code := cmd.Run([]string{
			"-cas", "0", "kv/write/cas", "bar=baz",
		})
		if code != 0 {
			t.Fatalf("expected 0 to be %d", code)
		}
		combined := ui.OutputWriter.String() + ui.ErrorWriter.String()
		if !strings.Contains(combined, "created_time") {
			t.Errorf("expected %q to contain %q", combined, "created_time")
		}

		ui, cmd = testKVPutCommand(t)
		cmd.client = client
		code = cmd.Run([]string{
			"-cas", "1", "kv/write/cas", "bar=baz",
		})
		if code != 0 {
			t.Fatalf("expected 0 to be %d", code)
		}
		combined = ui.OutputWriter.String() + ui.ErrorWriter.String()
		if !strings.Contains(combined, "created_time") {
			t.Errorf("expected %q to contain %q", combined, "created_time")
		}

		ui, cmd = testKVPutCommand(t)
		cmd.client = client
		code = cmd.Run([]string{
			"-cas", "1", "kv/write/cas", "bar=baz",
		})
		if code != 2 {
			t.Fatalf("expected 2 to be %d", code)
		}
		combined = ui.OutputWriter.String() + ui.ErrorWriter.String()
		if !strings.Contains(combined, "check-and-set parameter did not match the current version") {
			t.Errorf("expected %q to contain %q", combined, "check-and-set parameter did not match the current version")
		}

	})

	t.Run("v1_data", func(t *testing.T) {
		t.Parallel()

		client, closer := testVaultServer(t)
		defer closer()

		ui, cmd := testKVPutCommand(t)
		cmd.client = client

		code := cmd.Run([]string{
			"secret/write/data", "bar=baz",
		})
		if code != 0 {
			t.Fatalf("expected 0 to be %d", code)
		}
		combined := ui.OutputWriter.String() + ui.ErrorWriter.String()
		if !strings.Contains(combined, "Success!") {
			t.Errorf("expected %q to contain %q", combined, "created_time")
		}

		ui, rcmd := testReadCommand(t)
		rcmd.client = client
		code = rcmd.Run([]string{
			"secret/write/data",
		})
		if code != 0 {
			t.Fatalf("expected 0 to be %d", code)
		}
		combined = ui.OutputWriter.String() + ui.ErrorWriter.String()
		if strings.Contains(combined, "data") {
			t.Errorf("expected %q not to contain %q", combined, "data")
		}
	})

	t.Run("stdin_full", func(t *testing.T) {
		t.Parallel()

		client, closer := testVaultServer(t)
		defer closer()

		stdinR, stdinW := io.Pipe()
		go func() {
			stdinW.Write([]byte(`{"foo":"bar"}`))
			stdinW.Close()
		}()

		_, cmd := testKVPutCommand(t)
		cmd.client = client
		cmd.testStdin = stdinR

		code := cmd.Run([]string{
			"secret/write/stdin_full", "-",
		})
		if code != 0 {
			t.Fatalf("expected 0 to be %d", code)
		}

		secret, err := client.Logical().Read("secret/write/stdin_full")
		if err != nil {
			t.Fatal(err)
		}
		if secret == nil || secret.Data == nil {
			t.Fatal("expected secret to have data")
		}
		if exp, act := "bar", secret.Data["foo"].(string); exp != act {
			t.Errorf("expected %q to be %q", act, exp)
		}
	})

	t.Run("stdin_value", func(t *testing.T) {
		t.Parallel()

		client, closer := testVaultServer(t)
		defer closer()

		stdinR, stdinW := io.Pipe()
		go func() {
			stdinW.Write([]byte("bar"))
			stdinW.Close()
		}()

		_, cmd := testKVPutCommand(t)
		cmd.client = client
		cmd.testStdin = stdinR

		code := cmd.Run([]string{
			"secret/write/stdin_value", "foo=-",
		})
		if code != 0 {
			t.Fatalf("expected 0 to be %d", code)
		}

		secret, err := client.Logical().Read("secret/write/stdin_value")
		if err != nil {
			t.Fatal(err)
		}
		if secret == nil || secret.Data == nil {
			t.Fatal("expected secret to have data")
		}
		if exp, act := "bar", secret.Data["foo"].(string); exp != act {
			t.Errorf("expected %q to be %q", act, exp)
		}
	})

	t.Run("integration", func(t *testing.T) {
		t.Parallel()

		client, closer := testVaultServer(t)
		defer closer()

		_, cmd := testKVPutCommand(t)
		cmd.client = client

		code := cmd.Run([]string{
			"secret/write/integration", "foo=bar", "zip=zap",
		})
		if code != 0 {
			t.Fatalf("expected 0 to be %d", code)
		}

		secret, err := client.Logical().Read("secret/write/integration")
		if err != nil {
			t.Fatal(err)
		}
		if secret == nil || secret.Data == nil {
			t.Fatal("expected secret to have data")
		}
		if exp, act := "bar", secret.Data["foo"].(string); exp != act {
			t.Errorf("expected %q to be %q", act, exp)
		}
		if exp, act := "zap", secret.Data["zip"].(string); exp != act {
			t.Errorf("expected %q to be %q", act, exp)
		}
	})

	t.Run("no_tabs", func(t *testing.T) {
		t.Parallel()

		_, cmd := testKVPutCommand(t)
		assertNoTabs(t, cmd)
	})
}

func testKVGetCommand(tb testing.TB) (*cli.MockUi, *KVGetCommand) {
	tb.Helper()

	ui := cli.NewMockUi()
	return ui, &KVGetCommand{
		BaseCommand: &BaseCommand{
			UI: ui,
		},
	}
}

func TestKVGetCommand(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		out  string
		code int
	}{
		{
			"not_enough_args",
			[]string{},
			"Not enough arguments",
			1,
		},
		{
			"too_many_args",
			[]string{"foo", "bar"},
			"Too many arguments",
			1,
		},
		{
			"not_found",
			[]string{"secret/nope/not/once/never"},
			"",
			2,
		},
		{
			"default",
			[]string{"secret/read/foo"},
			"foo",
			0,
		},
		{
			"v1_field",
			[]string{"-field", "foo", "secret/read/foo"},
			"bar",
			0,
		},
		{
			"v2_field",
			[]string{"-field", "foo", "kv/read/foo"},
			"bar",
			0,
		},

		{
			"v2_not_found",
			[]string{"kv/nope/not/once/never"},
			"",
			2,
		},

		{
			"v2_read",
			[]string{"kv/read/foo"},
			"foo",
			0,
		},
		{
			"v2_read",
			[]string{"kv/read/foo"},
			"version",
			0,
		},
		{
			"v2_read_version",
			[]string{"--version", "1", "kv/read/foo"},
			"foo",
			0,
		},
	}

	t.Run("validations", func(t *testing.T) {
		t.Parallel()

		for _, tc := range cases {
			tc := tc

			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				client, closer := testVaultServer(t)
				defer closer()
				if err := client.Sys().Mount("kv/", &api.MountInput{
					Type: "kv-v2",
				}); err != nil {
					t.Fatal(err)
				}

				if _, err := client.Logical().Write("secret/read/foo", map[string]interface{}{
					"foo": "bar",
				}); err != nil {
					t.Fatal(err)
				}

				if _, err := client.Logical().Write("kv/data/read/foo", map[string]interface{}{
					"data": map[string]interface{}{
						"foo": "bar",
					},
				}); err != nil {
					t.Fatal(err)
				}

				ui, cmd := testKVGetCommand(t)
				cmd.client = client

				code := cmd.Run(tc.args)
				if code != tc.code {
					t.Errorf("expected %d to be %d", code, tc.code)
				}

				combined := ui.OutputWriter.String() + ui.ErrorWriter.String()
				if !strings.Contains(combined, tc.out) {
					t.Errorf("expected %q to contain %q", combined, tc.out)
				}
			})
		}
	})

	t.Run("no_tabs", func(t *testing.T) {
		t.Parallel()

		_, cmd := testKVGetCommand(t)
		assertNoTabs(t, cmd)
	})
}

func testKVMetadataGetCommand(tb testing.TB) (*cli.MockUi, *KVMetadataGetCommand) {
	tb.Helper()

	ui := cli.NewMockUi()
	return ui, &KVMetadataGetCommand{
		BaseCommand: &BaseCommand{
			UI: ui,
		},
	}
}

func TestKVMetadataGetCommand(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		out  string
		code int
	}{
		{
			"v1",
			[]string{"secret/foo"},
			"Metadata not supported on KV Version 1",
			1,
		},
		{
			"metadata_exists",
			[]string{"kv/foo"},
			"current_version",
			0,
		},
		{
			"versions_exist",
			[]string{"kv/foo"},
			"deletion_time",
			0,
		},
	}

	t.Run("validations", func(t *testing.T) {
		t.Parallel()

		for _, tc := range cases {
			tc := tc

			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				client, closer := testVaultServer(t)
				defer closer()
				if err := client.Sys().Mount("kv/", &api.MountInput{
					Type: "kv-v2",
				}); err != nil {
					t.Fatal(err)
				}

				if _, err := client.Logical().Write("kv/data/foo", map[string]interface{}{
					"data": map[string]interface{}{
						"foo": "bar",
					},
				}); err != nil {
					t.Fatal(err)
				}

				ui, cmd := testKVMetadataGetCommand(t)
				cmd.client = client

				code := cmd.Run(tc.args)
				if code != tc.code {
					t.Errorf("expected %d to be %d", code, tc.code)
				}

				combined := ui.OutputWriter.String() + ui.ErrorWriter.String()
				if !strings.Contains(combined, tc.out) {
					t.Errorf("expected %q to contain %q", combined, tc.out)
				}
			})
		}
	})

	t.Run("no_tabs", func(t *testing.T) {
		t.Parallel()

		_, cmd := testKVMetadataGetCommand(t)
		assertNoTabs(t, cmd)
	})
}

func testKVListCommand(tb testing.TB) (*cli.MockUi, *KVListCommand) {
	tb.Helper()

	ui := cli.NewMockUi()
	return ui, &KVListCommand{
		BaseCommand: &BaseCommand{
			UI: ui,
		},
	}
}

func TestKVListCommand(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		out  string
		code int
	}{
		{
			"not_enough_args",
			[]string{},
			"Not enough arguments (expected 1, got 0)",
			1,
		},
		{
			"too_many_args",
			[]string{"foo", "bar"},
			"Too many arguments (expected 1, got 2)",
			1,
		},
		{
			"invalid_depth",
			[]string{"-recursive", "-depth", "-2", "secret/foo"},
			"Invalid recursion depth: -2",
			1,
		},
		{
			"invalid_regexp",
			[]string{"-recursive", "-filter", "*", "secret/foo"},
			"Invalid regular expression: *",
			1,
		},
		{
			"invalid_concurrency",
			[]string{"-recursive", "-concurrent", "0", "secret/foo"},
			"Invalid concurrency value: 0",
			1,
		},
		{
			"not_found",
			[]string{"secret/nope/not/once/never"},
			"No value found at secret/nope/not/once/never",
			2,
		},
		{
			"default",
			[]string{"secret/list"},
			"bar/\nbaz/\nfoo/",
			0,
		},
		{
			"default_slash",
			[]string{"secret/list/"},
			"bar/\nbaz/\nfoo/",
			0,
		},
		{
			"recursive",
			[]string{"-recursive", "secret/list"},
			"secret/list/bar/\n" +
				"secret/list/bar/grault\n" +
				"secret/list/baz/\n" +
				"secret/list/baz/xyzzy/\n" +
				"secret/list/baz/xyzzy/thud\n" +
				"secret/list/baz/xyzzy/waldo\n" +
				"secret/list/foo/\n" +
				"secret/list/foo/grault/\n" +
				"secret/list/foo/grault/garply\n" +
				"secret/list/foo/qux/\n" +
				"secret/list/foo/qux/quux",
			0,
		},
		{
			"recursive_with_depth",
			[]string{"-recursive", "-depth", "2", "secret/list"},
			"secret/list/bar/\nsecret/list/baz/\nsecret/list/foo/",
			0,
		},
		{
			"recursive_with_filter",
			[]string{"-recursive", "-filter", "xyz+y", "secret/list"},
			"secret/list/baz/xyzzy/thud\nsecret/list/baz/xyzzy/waldo",
			0,
		},
		{
			"recursive_with_filter_depth",
			[]string{"-recursive", "-depth", "2", "-filter", "/f.*", "secret/list"},
			"secret/list/foo/",
			0,
		},
	}

	t.Run("validations", func(t *testing.T) {
		t.Parallel()

		for _, tc := range cases {
			tc := tc

			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				client, closer := testVaultServer(t)
				defer closer()

				keys := []string{
					"secret/list/foo/qux/quux",
					"secret/list/foo/grault/garply",
					"secret/list/bar/grault",
					"secret/list/baz/xyzzy/thud",
					"secret/list/baz/xyzzy/waldo",
				}
				for _, k := range keys {
					if _, err := client.Logical().Write(k, map[string]interface{}{
						"foo": "bar",
					}); err != nil {
						t.Fatal(err)
					}
				}

				ui, cmd := testKVListCommand(t)
				cmd.client = client

				code := cmd.Run(tc.args)
				if code != tc.code {
					t.Errorf("expected %d to be %d", code, tc.code)
				}

				combined := ui.OutputWriter.String() + ui.ErrorWriter.String()
				if !strings.Contains(combined, tc.out) {
					t.Errorf("expected %q to contain %q", combined, tc.out)
				}
			})
		}
	})

	t.Run("communication_failure", func(t *testing.T) {
		t.Parallel()

		client, closer := testVaultServerBad(t)
		defer closer()

		ui, cmd := testListCommand(t)
		cmd.client = client

		code := cmd.Run([]string{
			"secret/list",
		})
		if exp := 2; code != exp {
			t.Errorf("expected %d to be %d", code, exp)
		}

		expected := "Error listing secret/list/: "
		combined := ui.OutputWriter.String() + ui.ErrorWriter.String()
		if !strings.Contains(combined, expected) {
			t.Errorf("expected %q to contain %q", combined, expected)
		}
	})

	t.Run("no_tabs", func(t *testing.T) {
		t.Parallel()

		_, cmd := testListCommand(t)
		assertNoTabs(t, cmd)
	})
}
