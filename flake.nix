{
  description = "Solana Liquidity Indexer dev shell and CI checks";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        go = pkgs.go_1_22;
        commonBuildInputs = with pkgs; [
          go
          gnumake
          git
          pkg-config
          protobuf
          cmake
          clang
          nats-server
        ];
      in {
        devShells.default = pkgs.mkShell {
          name = "lp-indexer-dev-shell";
          packages = commonBuildInputs ++ (with pkgs; [
            gcc
            ninja
            openssl
          ]);
          shellHook = ''
            export GOFLAGS="${GOFLAGS:-}-buildvcs=false"
            export GOPATH="${GOPATH:-$PWD/.cache/go}"
            export GOCACHE="${GOCACHE:-$PWD/.cache/go-build}"
            export PATH="$GOPATH/bin:$PATH"
            echo "Go version: $(go version)"
          '';
        };

        checks.go-tests = pkgs.stdenv.mkDerivation {
          pname = "lp-indexer-go-tests";
          version = "unstable";
          src = ./.;
          buildInputs = commonBuildInputs;
          dontPatch = true;
          dontConfigure = true;
          dontInstall = true;
          buildPhase = ''
            runHook preBuild
            export HOME=$TMPDIR/home
            mkdir -p "$HOME"
            export GOPATH=$TMPDIR/go
            export GOCACHE=$TMPDIR/go-cache
            export GOFLAGS="-buildvcs=false"
            go test ./...
          '';
          installPhase = ''
            mkdir -p $out
          '';
        };
      }
    );
}
