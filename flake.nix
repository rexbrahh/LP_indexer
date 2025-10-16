{
  description = "Solana Liquidity Indexer dev shell and CI checks";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    nixpkgs-unstable.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, nixpkgs-unstable, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        pkgsUnstable = import nixpkgs-unstable { inherit system; };
        go = pkgsUnstable.go_1_24;

        goTools = with pkgs; [
          go
          golangci-lint
          buf
          protoc-gen-go
          protoc-gen-go-grpc
        ];

        cxxTools = with pkgs; [
          clang
          clang-tools
          cmake
          ninja
          pkg-config
          protobuf
        ];

        miscTools =
          (with pkgs; [
            git
            gnumake
            openssl
            direnv
            just
            nats-server
          ]) ++ pkgs.lib.optionals (!pkgs.stdenv.isDarwin) [ pkgs.gdb ];

        devToolchain = goTools ++ cxxTools ++ miscTools;

        mkDevShell = extraPackages: pkgs.mkShell {
          name = "lp-indexer-dev-shell";
          packages = devToolchain ++ extraPackages;
          shellHook = ''
            export GOFLAGS="${GOFLAGS:-}-buildvcs=false"
            export GOPATH="${GOPATH:-$PWD/.cache/go}"
            export GOCACHE="${GOCACHE:-$PWD/.cache/go-build}"
            export PATH="$GOPATH/bin:$PATH"
            export GOTOOLCHAIN=local
            echo "Go version: $(go version)"
          '';
        };

        natsDevScript = pkgs.writeShellScriptBin "nats-dev" ''
          exec ${pkgs.nats-server}/bin/nats-server -js "$@"
        '';

      in {
        devShells = {
          default = mkDevShell (with pkgs; [
            (python3.withPackages (ps: [ ps.pyyaml ]))
          ]);

          go = pkgs.mkShell {
            name = "lp-indexer-go-shell";
            packages = goTools ++ (with pkgs; [ git nats-server ]);
            shellHook = ''
              export GOFLAGS="${GOFLAGS:-}-buildvcs=false"
              export GOPATH="${GOPATH:-$PWD/.cache/go}"
              export GOCACHE="${GOCACHE:-$PWD/.cache/go-build}"
              export PATH="$GOPATH/bin:$PATH"
              export GOTOOLCHAIN=local
            '';
          };

          cpp = pkgs.mkShell {
            name = "lp-indexer-cpp-shell";
            packages = cxxTools
              ++ (with pkgs; [ lldb ])
              ++ pkgs.lib.optionals (!pkgs.stdenv.isDarwin) [ pkgs.gdb ];
            shellHook = ''
              export CC=clang
              export CXX=clang++
            '';
          };

          ci = pkgs.mkShell {
            name = "lp-indexer-ci-shell";
            packages = goTools ++ (with pkgs; [ pkg-config protobuf nats-server cmake clang ninja ]);
          };
        };

        packages = {
          nats-dev = natsDevScript;
        };

        apps = {
          nats-dev = {
            type = "app";
            program = "${natsDevScript}/bin/nats-dev";
          };
        };

        checks =
          let
            vendorHash = "sha256-Vymu2ejssgPNCgwx6l1QYv1AaTLy1SY7tnI4aK5EoDQ=";
          in {
            go-tests = pkgsUnstable.buildGoModule {
              pname = "lp-indexer-go-tests";
              version = "unstable";
              src = ./.;
              modRoot = ".";
              vendorHash = vendorHash;
              go = pkgsUnstable.go_1_24;
              subPackages = [ "./..." ];
              doCheck = true;
              nativeBuildInputs = [
                pkgs.nats-server
                pkgs.protobuf
                pkgs.gnumake
                pkgsUnstable.protoc-gen-go
                pkgsUnstable.protoc-gen-go-grpc
              ];
              modPostPatch = ''
                make proto-gen
              '';
              buildPhase = ''
                runHook preBuild
                runHook postBuild
              '';
              checkPhase = ''
                runHook preCheck
                make proto-gen
                export HOME=$TMPDIR/home
                mkdir -p "$HOME"
                export GOPATH=$TMPDIR/go
                export GOMODCACHE=$TMPDIR/go/pkg/mod
                export GOCACHE=$TMPDIR/go-cache
                export GOFLAGS="-buildvcs=false"
                export GOTOOLCHAIN=local
                go test ./...
                runHook postCheck
              '';
              installPhase = ''
                mkdir -p $out
              '';
            };

            go-tests-race = pkgsUnstable.buildGoModule {
              pname = "lp-indexer-go-tests-race";
              version = "unstable";
              src = ./.;
              modRoot = ".";
              vendorHash = vendorHash;
              go = pkgsUnstable.go_1_24;
              subPackages = [ "./..." ];
              doCheck = true;
              nativeBuildInputs = [
                pkgs.nats-server
                pkgs.protobuf
                pkgs.gnumake
                pkgsUnstable.protoc-gen-go
                pkgsUnstable.protoc-gen-go-grpc
              ];
              modPostPatch = ''
                make proto-gen
              '';
              buildPhase = ''
                runHook preBuild
                runHook postBuild
              '';
              checkPhase = ''
                runHook preCheck
                make proto-gen
                export HOME=$TMPDIR/home
                mkdir -p "$HOME"
                export GOPATH=$TMPDIR/go
                export GOMODCACHE=$TMPDIR/go/pkg/mod
                export GOCACHE=$TMPDIR/go-cache
                export GOFLAGS="-buildvcs=false"
                export GOTOOLCHAIN=local
                go test -race ./...
                runHook postCheck
              '';
              installPhase = ''
                mkdir -p $out
              '';
            };

          };
      }
    );
}
