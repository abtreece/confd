#!/bin/bash
set -ex

diff /tmp/confd-basic-test.conf test/integration/expect/basic.conf
if [[ ! -v VAULT_ADDR ]]; then
  diff /tmp/confd-exists-test.conf test/integration/expect/exists-test.conf
fi
diff /tmp/confd-iteration-test.conf test/integration/expect/iteration.conf
diff /tmp/confd-manykeys-test.conf test/integration/expect/basic.conf
diff /tmp/confd-nested-test.conf test/integration/expect/nested.conf

rm /tmp/confd-*;
