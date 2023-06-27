import argparse
import subprocess
import sys
import tempfile
import os

dir_path = os.path.dirname(os.path.realpath(__file__))
TIMECRAFT = os.path.join(dir_path, "..", "timecraft")

parser = argparse.ArgumentParser()
parser.add_argument("--version", action="store_true")
parser.add_argument("--test-file", action="store")
parser.add_argument("--arg", action="append", default=[])
parser.add_argument("--env", action="append", default=[])
parser.add_argument("--dir", action="append", default=[])

args = parser.parse_args()

if args.version:
    subprocess.run([TIMECRAFT, "version"])
    sys.exit(0)

TEST_FILE = args.test_file
PROG_ARGS = args.arg
ENV_ARGS = [j for i in args.env for j in ["--env", i]]
DIR_ARGS = [j for i in args.dir for j in ["--dir", i]]

status = 0
with tempfile.TemporaryDirectory() as tmpdir:
    config_file = os.path.join(tmpdir, "config.yaml")
    registry_dir = os.path.join(tmpdir, "registry")
    with open(config_file, "w") as f:
        f.write(f"registry:\n  location: {registry_dir}")

    r = subprocess.run([TIMECRAFT, "run", "--restrict", "--config", config_file] + ENV_ARGS + DIR_ARGS + [TEST_FILE] + PROG_ARGS, capture_output=True, text=True)
    status = r.returncode

    process_id = r.stderr.strip()

    r = subprocess.run([TIMECRAFT, "replay", "--config", config_file, process_id])
    if r.returncode != status:
       raise RuntimeError(f"timecraft replay exited with unexpected code {r.returncode}")

sys.exit(status)
