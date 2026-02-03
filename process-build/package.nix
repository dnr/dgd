{ stdenvNoCC, go }:
stdenvNoCC.mkDerivation {
  pname = "dgd-process-build";
  version = "0.0.1";
  src = ./.;
  buildInputs = [ go ];
  buildPhase = "GOTOOLCHAIN=local GOCACHE=$TMPDIR GOBIN=$out/bin go install .";
}
