#!/bin/bash
# Prove how many times redis writes its RDB when a pod is replaced.
#
# With the preStop hook attached (operator-managed mode, before the fix) the
# sequence is:
#     preStop  -> redis-cli save     -> "DB saved on disk"   <-- BEFORE SIGTERM
#     SIGTERM  -> shutdown save      -> "DB saved on disk"   <-- AFTER  SIGTERM
# so the dataset is written twice for every pod replacement.
#
# With the hook gated to Sentinel mode only, the first write disappears and the
# SIGTERM save alone remains.
#
# The discriminator is position, not count: a save logged *before* the instance
# manager reports the signal can only have come from preStop, because nothing
# else runs then.
#
# Pod logs die with the pod (a StatefulSet delete recreates rather than
# restarts, so --previous is empty), so the log is streamed during termination.
#
# usage: prove-shutdown-saves.sh <rf-name> [namespace]
set -u
export KUBECONFIG=${KUBECONFIG:-$HOME/.kube/kubeconfig-us-east-1b}
N=${1:?usage: prove-shutdown-saves.sh <rf-name> [namespace]}
NS=${2:-cachetogo}
OUT=$(mktemp -t shutdownlog)

fail() { echo "  ABORT: $*" >&2; exit 1; }

PW=$(kubectl get secret "${N}-user" -n "$NS" -o jsonpath='{.data.password}' 2>/dev/null | base64 -d)
[ -n "$PW" ] || fail "no password secret ${N}-user in $NS"

# Only ever touch a replica. Replacing the master would trigger a handover and
# muddy the measurement with an unrelated failover.
TARGET=""
for p in "rfr-$N-0" "rfr-$N-1"; do
  role=$(kubectl exec -n "$NS" "$p" -- redis-cli -a "$PW" --no-auth-warning info replication 2>/dev/null | grep -c '^role:master')
  [ "$role" = "0" ] && { TARGET=$p; break; }
done
[ -n "$TARGET" ] || fail "could not identify a replica pod (is the cluster healthy?)"
echo "  target replica : $TARGET"

# Redis only saves on SIGTERM when there is something to save, so make the
# dataset dirty. Otherwise a clean shutdown legitimately writes nothing and the
# test proves nothing.
kubectl exec -n "$NS" "rfr-$N-0" -- redis-cli -a "$PW" --no-auth-warning \
  set "shutdown-probe:$(date +%s)" 1 >/dev/null 2>&1
echo "  dirtied dataset so a save is guaranteed"

echo "  streaming logs, then deleting the pod..."
kubectl logs -n "$NS" "$TARGET" -c redis -f --tail=1 > "$OUT" 2>/dev/null &
LOGPID=$!
sleep 2
kubectl delete pod -n "$NS" "$TARGET" --wait=false >/dev/null 2>&1

for _ in $(seq 1 60); do
  kill -0 $LOGPID 2>/dev/null || break
  sleep 1
done
kill $LOGPID 2>/dev/null
wait $LOGPID 2>/dev/null

# The instance manager announces the signal; anything logged before that line
# happened while the pod was still running normally, i.e. in preStop.
SIGLINE=$(grep -n -m1 -E 'redis-instance: received signal|Received SIGTERM' "$OUT" | cut -d: -f1)
SAVES_TOTAL=$(grep -c 'DB saved on disk' "$OUT")
if [ -n "$SIGLINE" ]; then
  SAVES_BEFORE=$(head -n "$((SIGLINE - 1))" "$OUT" | grep -c 'DB saved on disk')
  SAVES_AFTER=$(tail -n "+$SIGLINE" "$OUT" | grep -c 'DB saved on disk')
else
  SAVES_BEFORE=0; SAVES_AFTER=$SAVES_TOTAL
fi

echo
echo "  === shutdown log ==="
grep -E 'DB saved on disk|redis-instance:|Received SIGTERM|User requested shutdown|Saving the final|ready to exit' "$OUT" \
  | sed 's/^/    /'
echo
echo "  === result ==="
echo "    saves before the signal (preStop) : $SAVES_BEFORE"
echo "    saves after  the signal (SIGTERM) : $SAVES_AFTER"
if [ "$SAVES_BEFORE" -ge 1 ] && [ "$SAVES_AFTER" -ge 1 ]; then
  echo "    VERDICT: DOUBLE WRITE - preStop saved, then SIGTERM saved again"
elif [ "$SAVES_BEFORE" -eq 0 ] && [ "$SAVES_AFTER" -ge 1 ]; then
  echo "    VERDICT: SINGLE WRITE - only the SIGTERM save ran"
elif [ -z "$SIGLINE" ]; then
  echo "    VERDICT: INCONCLUSIVE - no signal marker in the captured log"
else
  echo "    VERDICT: INCONCLUSIVE - no save recorded at all (dataset clean?)"
fi
echo "    raw log: $OUT"
