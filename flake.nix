{
  description = "gordyclark.com — a static essay site generator (Go + D2), no server, no JavaScript.";

  inputs = {
    # Pinned to nixpkgs-unstable (flake.lock records the exact revision) because
    # this project's go.mod requires Go 1.26, newer than the 25.05 stable
    # channel ships. flake.lock keeps the Go toolchain and d2 reproducible.
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };

        # Runtime tools. d2 is the only non-Go runtime dependency of the build;
        # its version is pinned transitively via flake.lock.
        d2 = pkgs.d2;
        go = pkgs.go;

        # The static-site build as a derivation. `nix build` produces $out = the
        # rendered ./static tree, with NO network access at build time (d2 runs
        # locally; all Go deps are vendored via vendorHash).
        site = pkgs.buildGoModule {
          pname = "gordyclark-com";
          version = "0.1.0";
          src = ./.;

          # Regenerate with: nix build 2>&1 (it prints the correct hash on mismatch),
          # or `nix run nixpkgs#nix-prefetch -- ...`. Placeholder triggers the hint.
          vendorHash = "sha256-ggZFW+Fa9WSN8NVHTZmsLWGeulBX7rTU78+z7jdYPrc=";

          # d2 must be on PATH during the build so cmd/render can invoke it.
          nativeBuildInputs = [ d2 ];

          # We do not want the default `go build` of all packages as the output;
          # instead we run cmd/render to emit the static site, then install it.
          buildPhase = ''
            runHook preBuild
            export HOME=$TMPDIR
            # Render into a local ./static using the checked-in content/assets/templates.
            go run ./cmd/render --content ./content --out ./static --cache ./.cache
            runHook postBuild
          '';

          installPhase = ''
            runHook preInstall
            mkdir -p $out
            cp -r ./static/. $out/
            runHook postInstall
          '';

          # No test phase here; `just test` covers tests in the dev shell.
          doCheck = false;
        };
      in
      {
        packages.default = site;
        packages.site = site;

        devShells.default = pkgs.mkShell {
          packages = [
            go
            d2
            pkgs.just
            pkgs.rclone
            pkgs.wrangler
            pkgs.gopls
          ];
          shellHook = ''
            echo "gordyclark.com dev shell — go $(go version | cut -d' ' -f3), d2 $(d2 --version 2>/dev/null || echo '?')"
            echo "recipes: just build | just hydrate <file> | just deploy | just deploy-api | just test"
          '';
        };
      });
}
