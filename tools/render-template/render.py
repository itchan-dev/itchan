import os, sys
from jinja2 import Environment, StrictUndefined

env = Environment(undefined=StrictUndefined, keep_trailing_newline=True)
open(sys.argv[2], "w").write(env.from_string(open(sys.argv[1]).read()).render(**os.environ))
