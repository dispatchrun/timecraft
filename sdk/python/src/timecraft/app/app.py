import os
from typing import Optional, Callable, Any, Dict
import hashlib
import cloudpickle
import aiohttp.web
import pickle
import inspect

from timecraft import TaskRequest, TaskResponse, TaskState, ModuleSpec, Client
from timecraft import HTTPRequest


class Entrypoint:
    """
    Each App has a single entrypoint. This is the equivalent of a main function
    or _start in WASM.
    """
    _func: Callable[..., Any]
    _app: "App"

    def __init__(self, fn, app):
        self._func = fn
        self._app = app

    def __call__(self, *args, **kwargs):
        return self._func(*args, **kwargs)


class Promise:
    """
    A Promise is used to track the status of a task or a function.
    """
    _app: "App"
    _tid: str
    _addr: str
    _resp: Optional[TaskResponse]
    _done: bool

    def __init__(self, app, tid):
        self._app = app
        self._tid = tid
        self._done = False

    async def result(self) -> Optional[TaskResponse]:
        """
        Returns the result of the task, or None if the task is not done.
        The result is represented by a timecraft.TaskResponse object.
        """
        await self._update()
        return self._resp

    async def is_done(self) -> bool:
        """
        Returns True if the task is done, False otherwise.
        """
        await self._update()
        return self._done

    async def _update(self):
        # TOOD: move to TaskState
        def _is_done(state) -> bool:
            done = [TaskState.SUCCESS, TaskState.ERROR]
            return state in done

        tasks = self._app._client.lookup_tasks([self._tid])

        if len(tasks) > 1:
            raise Exception("too many tasks returned")
        if len(tasks) == 0:
            raise Exception(f"task {self._tid} not found")

        task = tasks[0]
        if _is_done(task.state):
            self._done = True

        self._resp = task


class Function:
    """
    A Function is the execution unit of a timecraft App. Behind the scene,
    a function will be scheduled as a task by the Ring scheduler.
    """
    _func: Callable[..., Any]
    _app: "App"
    _name: str

    def __init__(self, app: "App", name: str, f: Callable[..., Any]) -> None:
        self._func = f
        self._app = app
        self._name = name

    def __call__(self, *args):
        return self._func(*args)

    def call(self, *args, **kwargs) -> Promise:
        """
        Calls the function with the given arguments and returns a Promise.
        """
        return self._app._spawn_task(self._name, *args, **kwargs)


class App:
    """
    App is the main class of the Timecraft Python engine. An App will be used
    to register a set of functions to be scheduled by the Ring scheduler.
    """
    _name: Optional[str]
    _client: Client

    _start: Optional[Entrypoint]
    _functions: Dict[str, Function]

    _web: aiohttp.web.Application

    _source: Optional[str]

    def __init__(self) -> None:
        self._tasks = {}
        self._client = Client()
        self._functions = {}
        self._start = None

        # TODO: move somewhere else
        self._web = aiohttp.web.Application()
        self._web.add_routes([aiohttp.web.post("/", self._handle)])

        # TODO: this is not reliable
        self._source = inspect.stack()[1].filename

    async def _serve(self):
        # TODO: handle failures
        await aiohttp.web._run_app(app=self._web, host="0.0.0.0", port=3000, print=None)

    # TODO: ensure the HTTP server can be gracefully shutted down once
    # the main function is closed.
    async def run(self, args=None):
        """
        Run the app. If no arguments are passed, the entrypoint function will
        be executed. Otherwise, the app will start an HTTP server waiting for
        task to be submitted.
        """
        if args is None or len(args) == 0:
            if self._start is not None: return await self._start()
            return

        await self._serve()

    def function(self, name: Optional[str] = None) -> Callable[..., Function]:
        """
        Function decorator used to register a function to the app.
        """
        def decorator(func: Callable[..., Any]) -> Function:
            if name:
                fid = name
            else:
                m = hashlib.sha256()
                m.update(cloudpickle.dumps(func))
                fid = m.hexdigest()

            f = Function(self, fid, func)
            self._functions[fid] = f
            self._web.add_routes([aiohttp.web.post(f"/{fid}", self._handle)])
            return f
        return decorator

    def start(self) -> Callable[..., Any]:
        """
        Decorator used to register the entrypoint function of the app.
        Raise an exception is called multiple times.
        """
        if self._start is not None:
            raise Exception("start function already defined")

        def decorator(func: Callable[..., Any]) -> Callable[..., Any]:

            self._start = Entrypoint(func, self)
            return func
        return decorator

    def _spawn_task(self, name: str, *args, **kwargs) -> Promise:
        workerfile = self._source
        if workerfile is None:
            workerfile = os.path.abspath(__file__)

        spec = ModuleSpec(args=[workerfile, name])

        input = pickle.dumps(args)

        # TODO: serialize args and kwargs into input
        reqs = [TaskRequest(module=spec, input=HTTPRequest(
            method="POST",
            path=f"/{name}",
            port=3000,
            body=input,
            headers={},
        ))]

        tids = self._client.submit_tasks(reqs)
        return Promise(self, tids[0])

    # https://docs.aiohttp.org/en/stable/web_reference.html#request-and-base-request
    async def _handle(self, request):
        s = request.path.split("/")
        if len(s) < 2:
            return aiohttp.web.Response(status=400, body=b"invalid request")

        fname = s[1]
        fn = self._functions[fname]
        if fn is None:
            return aiohttp.web.Response(status=404, body=b"function not found")

        data = await request.read()
        input = pickle.loads(data)

        try:
            res = fn.__call__(*input)
        except TypeError as e:
            return aiohttp.web.Response(status=500, body=f"{e}")

        output = pickle.dumps(res)
        return aiohttp.web.Response(status=200, body=output)

