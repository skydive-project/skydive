{ pkgs ? import <nixpkgs> {}
, nixpkgsContrailPath ? pkgs.fetchFromGitHub {
    owner = "nlewo";
    repo = "nixpkgs-contrail";
    rev = "d1bcfa3527e838b6cbc542b4aac5a8dd47db90e3";
    sha256 = "03kp134rbc7mmxf7vhbhp80a1dacbyj3q0db5mg3w7wmrkwvjmlc";
  }
}:

let
  contrailPkgs = import nixpkgsContrailPath {};

  skydiveTestFunctionals = ../../tests/functionals;

  testScript = ''
    # This is to wait until OpenContrail is ready
    $machine->waitUntilSucceeds("curl -s http://localhost:8083/Snh_ShowBgpNeighborSummaryReq | grep machine | grep -q Established");
    $machine->succeed("${skydiveTestFunctionals} -test.run TestOpenContrail -standalone -agent.topology.probes=opencontrail");
  '';

in 
  contrailPkgs.contrail32.test.allInOne.override { inherit testScript; }
  // { inherit skydiveTestFunctionals; }
