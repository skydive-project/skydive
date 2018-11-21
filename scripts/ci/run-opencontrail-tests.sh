#!/usr/bin/env bash

set -e

# Install Nix if not present
if ! type -p nix >/dev/null
then
    curl https://nixos.org/nix/install | sh
    . $HOME/.nix-profile/etc/profile.d/nix.sh
fi

make test.functionals.static TAGS="$TAGS opencontrail_tests" WITH_LIBVIRT=false

# For development or debug purposes, the Contrail VM can
# be launched in an interactive mode.
# $ nix-build scripts/ci/opencontrail-tests.nix -A driver
# This creates scripts that just run the VM. It is then possible to
# ssh it to manually run the test suite.
# For more information: https://nixos.org/nixos/manual/index.html#sec-running-nixos-tests
nix-build scripts/ci/opencontrail-tests.nix --no-out-link \
  --substituters "https://opencontrail.cachix.org https://cache.nixos.org" \
  --option trusted-public-keys "opencontrail.cachix.org-1:QCsZCHATxFVRRHdilxc1qiV2QmiPV02NTs10oVkf+UQ= cache.nixos.org-1:6NCHdD59X431o0gWypbMrAURkbJ16ZPMQFGspcDShjY="

# Remove the test script from the store to save space
nix-store --delete $(nix-build scripts/ci/opencontrail-tests.nix --no-out-link -A skydiveTestFunctionals)
