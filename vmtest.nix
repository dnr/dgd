{ pkgs, exampleDepProxyHash }:
let
  testsrc = pkgs.lib.sourceByRegex ./. [ "^[a-zA-Z0-9].*" ];
  example_dep_proxy_data =
    pkgs.runCommand "dgd-vmtest-proxy-data"
      {
        inherit (pkgs) go;
        src = ./examples/dep;
        outputHash = exampleDepProxyHash;
        outputHashMode = "recursive";
      }
      ''
        cd $src
        export GOTOOLCHAIN=local
        export GOPATH=$TMPDIR/go
        $go/bin/go mod download -x
        cp -r $GOPATH/pkg/mod/cache/download $out
      '';
in
pkgs.testers.runNixOSTest {
  name = "dgd-vmtest";
  nodes.machine =
    { pkgs, ... }:
    {
      # give more resources:
      virtualisation.memorySize = 4096;
      virtualisation.cores = 4;
      virtualisation.diskSize = 10240;
      # faster startup:
      virtualisation.useNixStoreImage = true;
      virtualisation.writableStore = true;

      virtualisation.additionalPaths = [
        # include nix sources.
        # hack to avoid re-copying nixpkgs and source to store.
        # pkgs must have been imported from a store path.
        (builtins.storePath pkgs.path)
        (builtins.storePath testsrc)
        # include deps so that we can build without network:
        pkgs.go
        pkgs.stdenv
        pkgs.stdenvNoCC
      ];

      # fail fast if we missed something
      nix.settings.substituters = pkgs.lib.mkForce [ ];
      nix.settings.hashed-mirrors = pkgs.lib.mkForce [ ];
      nix.settings.connect-timeout = 1;
      # force proxy data into build env (simpler than using network)
      nix.settings.sandbox-paths = [ example_dep_proxy_data ];

      # use latest version and turn on dyndrv features
      nix.package = pkgs.nixVersions.nix_2_32;
      nix.settings = {
        experimental-features = [
          "nix-command"
          "dynamic-derivations"
          "ca-derivations"
          "recursive-nix"
        ];
      };
    };
  testScript = ''
    machine.succeed("""
      for dyn in true false; do
        echo "====== building with dyn=$dyn"
        nix-build \
          ${toString testsrc} \
          --no-out-link \
          -A testExamples \
          --arg pkgs 'import ${toString pkgs.path} { }' \
          --argstr goproxy file:///${example_dep_proxy_data} \
          --arg useDynDrv $dyn
      done
    """)
  '';
}
