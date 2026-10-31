"""Bind only host-discovered trusted Skills. Instructions remain lazy."""
import importlib
import keyword
import os
import importlib.util
from pathlib import Path
import tomllib
import inspect
import sys

RESERVED = {'workbench', 'mcp', 'memory', 'asyncio', '__builtins__', '_'}
HOST_HANDLERS = {'mcp.config', 'mcp.list_connections'}


class CallableSkill:
    def __init__(self, module, record=None, usage=None):
        self.module = module
        self.__doc__ = module.run.__doc__
        self.__signature__ = inspect.signature(module.run)
        self.record = record
        self.usage = usage

    def _observe(self, outcome, value):
        if self.record is not None and self.usage is not None:
            try:
                text = repr(value)[:4096]
            except BaseException:
                text = '<unrepresentable result>'
            self.usage(dict(name=self.record['name'], revision=self.record['revision'],
                            publication=self.record.get('publication'), outcome=outcome, result=text))

    def __call__(self, *args, **kwargs):
        if inspect.iscoroutinefunction(self.module.run):
            async def invoke():
                try:
                    value = await self.module.run(*args, **kwargs)
                except BaseException as exc:
                    self._observe('error', exc)
                    raise
                self._observe('ok', value)
                return value
            return invoke()
        try:
            value = self.module.run(*args, **kwargs)
        except BaseException as exc:
            self._observe('error', exc)
            raise
        self._observe('ok', value)
        return value

    def __getattr__(self, name):
        return getattr(self.module, name)


def load(skills, namespace, usage=None):
    inventory = []
    for skill in skills:
        record = dict(skill, state='instructions')
        inventory.append(record)
        metadata = Path(skill['file']).parent / 'pyproject.toml'
        if not metadata.is_file():
            continue
        try:
            with metadata.open('rb') as stream:
                project = tomllib.load(stream)
            options = project.get('tool', {}).get('vivy', {}).get('skill', {})
            name = options.get('module', project['project']['name'].replace('-', '_'))
            binding = options.get('binding', name)
            if not binding.isidentifier() or keyword.iskeyword(binding) or binding in RESERVED or binding in namespace:
                raise ValueError(f'Unavailable or reserved binding: {binding}')
            if set(options.get('requires_host', [])) - HOST_HANDLERS:
                raise ValueError('Skill requires an unavailable host handler')
            managed = bool(skill.get('revision'))
            source = metadata.parent / 'src'
            if managed:
                sys.path.insert(0, str(source))
            module = importlib.import_module(name)
            if managed:
                origin = Path(module.__file__).resolve()
                if not origin.is_relative_to(source.resolve()):
                    raise ValueError(f'Module origin does not match frozen Skill: {origin}')
                for imported_name, imported in list(sys.modules.items()):
                    if imported_name.startswith(name + '.'):
                        origin = getattr(imported, '__file__', None)
                        if origin is None or not Path(origin).resolve().is_relative_to(source.resolve()):
                            raise ValueError(f'Submodule origin does not match frozen Skill: {imported_name}')
                if not callable(getattr(module, 'run', None)):
                    raise ValueError('Managed Skill must expose a callable run entry point')
            namespace[binding] = CallableSkill(module, record if managed else None, usage) if callable(getattr(module, 'run', None)) else module
            record.update(state='ready', binding=binding, module=name)
        except Exception as exc:
            # Broken trusted code must not hide the remaining usable capabilities.
            record.update(state='unavailable', error=f'{type(exc).__name__}: {str(exc)[:512]}')
    return inventory


class Workbench:
    def __init__(self, inventory):
        self._inventory = inventory
        self.readiness = {}

    def list_skills(self):
        return [dict(row) for row in self._inventory]

    def instructions(self, name):
        match = next(row for row in self._inventory if row['name'] == name)
        return Path(match['file']).read_text(encoding='utf-8')

    @staticmethod
    def artifact(value, media_type='text/plain'):
        from .artifacts import save
        return save(value, media_type)


def memory_status(namespace):
    """Observe the current binding only; never query the database for the footer."""
    if not os.environ.get('VIVY_WORKBENCH_MEMORY_ROOT'):
        return 'off'
    try:
        from .memory import Memory
        memory = namespace.get('memory')
        # Users may rebind names. Observation must not invoke their descriptors
        # or change the success/failure of the code cell being reported.
        if type(memory) is not Memory:
            return 'error'
        closed = object.__getattribute__(memory, '__dict__').get('closed')
        return 'closed' if closed is True else 'ready' if closed is False else 'error'
    except Exception:
        return 'error'


def bootstrap(namespace, skills, usage=None):
    from . import mcp
    import asyncio
    namespace.update(mcp=mcp, asyncio=asyncio)
    namespace['workbench'] = Workbench([])
    namespace['workbench'].readiness['mcp'] = 'ready' if importlib.util.find_spec('mcp') else 'run /python prepare'
    root = os.environ.get('VIVY_WORKBENCH_MEMORY_ROOT')
    if root:
        try:
            from .memory import Memory
            namespace['memory'] = Memory(root)
            namespace['workbench'].readiness['memory'] = 'ready'
        except Exception as exc:
            namespace['workbench'].readiness['memory'] = f'{type(exc).__name__}: {str(exc)[:512]}'
    inventory = load(skills, namespace, usage=usage)
    namespace['workbench']._inventory = inventory
    return inventory
