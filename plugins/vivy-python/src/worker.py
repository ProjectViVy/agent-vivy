"""Development entry point; installed workers use -m vivy_workbench.repl."""
import pathlib
import sys
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1] / 'python'))
from vivy_workbench.repl import main
if __name__ == '__main__':
    main()
