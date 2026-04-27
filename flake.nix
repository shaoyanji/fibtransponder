{
  description = "Fibtransponder: Fibonacci-radix streaming state machine with proprioceptive calibration";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        # Match go.mod: go 1.25.7 → use go_1_25 from nixpkgs
        goPkg = pkgs.go_1_25;
      in
      {
        devShells.default = pkgs.mkShell {
          # Ensure static builds without cgo, and provide fonts for typst without touching the system
          CGO_ENABLED = "0";
          nativeBuildInputs = [ goPkg fontconfig tex-gyre inconsolata jetbrains-mono gsfonts ];
          shellHook = ''
            export CGO_ENABLED=0
            export GOFLAGS="-modcacherw"
            echo "=== ${self.description} dev shell ==="
            echo "Go: $(go version)"
            echo "CGO_ENABLED=$CGO_ENABLED"
            echo "Running 'go test ./... -count=1' will now work without gcc."
          '';
        };

        # Optional: build all command binaries (static)
        packages.default = pkgs.buildGoModule {
          pname = "fibtransponder";
          version = "0.1.1";
          src = ./.;
          CGO_ENABLED = 0;
          go = goPkg;
          # Build each cmd binary into $out/bin
          buildPhase = ''
            runHook preBuild
            for cmd in $(go list ./cmd/...); do
              binname=$(basename $cmd)
              go build -o $out/bin/$binname ./cmd/$binname
            done
            runHook postBuild
          '';
          # Don't run tests in install phase
          installPhase = ''
            runHook preInstall
            # Binaries already placed in $out/bin by buildPhase
            runHook postInstall
          '';
          # Allow network during build to fetch modules (go.sum is committed so it's reproducible)
          # Nix will cache the mod download.
          modDownloadPhase = true;
        };

        # Convenience: individual command packages
        packages.api = pkgs.buildGoModule {
          pname = "fibtransponder-api";
          version = "0.1.1";
          src = ./.;
          CGO_ENABLED = 0;
          go = goPkg;
          buildPhase = ''
            runHook preBuild
            go build -o $out/bin/fibtransponder-api ./cmd/api
            runHook postBuild
          '';
          installPhase = "runHook preInstall; runHook postInstall";
        };

        packages.fibcompress = pkgs.buildGoModule {
          pname = "fibcompress";
          version = "0.1.1";
          src = ./.;
          CGO_ENABLED = 0;
          go = goPkg;
          buildPhase = ''
            runHook preBuild
            go build -o $out/bin/fibcompress ./cmd/fibcompress
            runHook postBuild
          '';
          installPhase = "runHook preInstall; runHook postInstall";
        };

        packages.fibtransponder = pkgs.buildGoModule {
          pname = "fibtransponder-cli";
          version = "0.1.1";
          src = ./.;
          CGO_ENABLED = 0;
          go = goPkg;
          buildPhase = ''
            runHook preBuild
            go build -o $out/bin/fibtransponder ./cmd/fibtransponder
            runHook postBuild
          '';
          installPhase = "runHook preInstall; runHook postInstall";
        };

        packages.tui = pkgs.buildGoModule {
          pname = "fibtransponder-tui";
          version = "0.1.1";
          src = ./.;
          CGO_ENABLED = 0;
          go = goPkg;
          buildPhase = ''
            runHook preBuild
            go build -o $out/bin/fibtransponder-tui ./cmd/tui
            runHook postBuild
          '';
          installPhase = "runHook preInstall; runHook postInstall";
        };

        # Expose a convenient formatter or checks
        checks = {
          test = pkgs.runCommand "fibtransponder-tests" {
            nativeBuildInputs = [ goPkg ];
            CGO_ENABLED = "0";
          } ''
            mkdir $out
            go test ./... -count=1 -timeout=180s
            touch $out/done
          '';
        };
      });
}
