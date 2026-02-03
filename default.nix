{
  # TODO: switch back to nixos stable when it has nix 2.32.5
  pkgs ? import (builtins.fetchTarball {
    url = "https://releases.nixos.org/nixos/unstable/nixos-26.05pre933481.c5296fdd05cf/nixexprs.tar.xz";
    sha256 = "sha256:07j4pqiv6w0grka3s2qyqw8v3826kfyix81wz92lnj75gm64qmcy";
  }) { },
  useDynDrv ? false,
  goproxy ? null,
}:
let
  builders = {
    # This is the public interface so far:
    buildWithIFD = import ./ifd.nix { inherit pkgs; };
    buildWithDynDrv = import ./dyn.nix { inherit pkgs; };
  };

  # vendorHash = "sha256-tPfVTwRGWoj+mg/71qtkQ9yzdeebTEwT+x5iBSpVYs0=";
  exampleDepVendorHash = "sha256-KxNlj83vZFdjxrCn2DN5FX9QiTKOxBXZ6yvAYaVvQbY=";
  exampleDepProxyHash = "sha256-Ek6/dzhmbaJXJSMcJh8m71XClvdvDoiBo3NwAcFuM3w=";
  exampleBuilder = if useDynDrv then builders.buildWithDynDrv else builders.buildWithIFD;
  examples = {
    example_hello = exampleBuilder {
      src = ./examples/hello;
    };
    example_hello_nocgo = exampleBuilder {
      src = ./examples/hello;
      env.CGO_ENABLED = 0;
    };
    example_dep = exampleBuilder {
      src = ./examples/dep;
      vendorHash = exampleDepVendorHash;
      env = if goproxy != null then { GOPROXY = goproxy; } else { };
    };
    example_cgo = exampleBuilder {
      src = ./examples/cgo;
      env.CGO_ENABLED = 1;
    };
    example_packages = exampleBuilder {
      src = ./examples/packages;
    };
    example_storepath = exampleBuilder {
      # using cleanSource forces src into the store early, this changes some code paths
      src = pkgs.lib.cleanSource ./examples/packages;
    };
    example_subpackage = exampleBuilder {
      src = ./examples/subpackage;
      subPackage = "cmd/hello";
    };
  };

  testers = {
    testExamples =
      pkgs.runCommandLocal "dgd-run-test-examples"
        {
          exampleNames = builtins.attrValues examples;
          __contentAddressed = useDynDrv; # needed to depend on a dynamic derivation?
        }
        ''
          set -e
          for ex in $exampleNames; do
            echo "=== running $ex"
            o="$($ex)"
            echo "--- output: $o"
            [[ $o = "Hello, DGD!" ]]
          done
          touch $out
        '';

    vmtest = import ./vmtest.nix { inherit pkgs exampleDepProxyHash; };
  };

in
builders // examples // testers
