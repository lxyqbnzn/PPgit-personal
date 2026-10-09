# PPGit Go Source

Copyright (C) Electric Reverse
SPDX-License-Identifier: AGPL-3.0-or-later

This source distribution contains the Go application and its embedded offline UI.
Go 1.25+ is required. From this directory:

    go run ./cmd/ppgit

Or build and run (Windows):

    go build -trimpath -o bin/ppgit.exe ./cmd/ppgit
    .\bin\ppgit.exe

macOS / Linux:

    go build -trimpath -o bin/ppgit ./cmd/ppgit
    ./bin/ppgit

The default browser address is http://127.0.0.1:9300.
Runtime data is created separately under data/. Use -data to override it.
Use the executable's -stop flag to request a graceful shutdown.

First-party code is licensed under the GNU Affero General Public License,
version 3 or (at your option) any later version. See LICENSE and COPYRIGHT.
Third-party components retain their original licenses; see THIRD_PARTY_NOTICES.md,
LICENSES/, and web/static/vendor/lucide/LICENSE.

Tests, reports, developer tooling, binaries, caches and runtime data are excluded.
Ordinary first-party source comments are removed. Compiler directives and legal
notices are retained. Third-party assets are unchanged. Exactly 20 copyright
comments are placed at randomly selected Go function-body starts.
