import base64
from typing import List, Optional
from pprint import pprint
from enum import Enum
import dataclasses
from dataclasses import dataclass
from .http import Session
import requests

def remap(d, before, after, f=None):
    if before in d:
        d[after] = d.pop(before)
        if f:
            d[after]=f(d[after])
    return d

class TaskState(Enum):
    UNSPECIFIED = "TASK_STATE_UNSPECIFIED"
    QUEUED = "TASK_STATE_QUEUED"
    INITIALIZING = "TASK_STATE_INITIALIZING"
    EXECUTING = "TASK_STATE_EXECUTING"
    ERROR = "TASK_STATE_ERROR"
    SUCCESS = "TASK_STATE_SUCCESS"


ProcessID = str
TypeID = str
Header = dict[str, str]
TaskID = str

@dataclass
class ModuleSpec:
    path: str
    args: list[str]

@dataclass
class TaskInput:
    def serialize(self) -> dict[str,any]:
        raise NotImplementedError

@dataclass
class TaskOutput:
    @classmethod
    def deserialize(cls, data: dict[str,any]):
        raise NotImplementedError

@dataclass
class TaskRequest:
    module: ModuleSpec
    input: TaskInput

@dataclass
class TaskResponse:
    taskID: TaskID
    state: TaskState
    error: Optional[str] = None
    output: Optional[TaskOutput] = None
    processID: Optional[ProcessID] = None

@dataclass
class HTTPRequest(TaskInput):
    method: str
    path: str
    headers: Header
    body: bytes

    def serialize(self):
        headers = []
        for k, v in self.headers:
            headers.append({
                "name": k,
                "value": v,
            })
        http_request = {
            "method": self.method,
            "path": self.path,
            "headers": headers,
            "body": self.body,
        }
        if self.body is not None:
            remap(http_request, "body", "body", lambda body: base64.b64encode(body).decode("utf-8"))
        return {
            "httpRequest": http_request
        }

def zipheader(lst):
    return dict((x["name"], x["value"]) for x in lst)
    
@dataclass
class HTTPResponse(TaskOutput):
    status_code: int
    headers: Header
    body: bytes

    @classmethod
    def deserialize(cls, data):
        remap(data, "statusCode", "status_code")
        remap(data, "headers", "headers", zipheader)
        remap(data, "body", "body", base64.b64decode)
        return cls(**data)


class Client:
    """
    Client to interface with the Timecraft server.
    """

    _root = "http+unix://\0timecraft.sock"
    
    def __init__(self):
        self.session = Session()

    def _rpc(self, endpoint, payload):
        r = self.session.post(self._root+"/timecraft.server.v1.TimecraftService/" + endpoint, json=payload)
        out = r.json()

        try:
            r.raise_for_status()
        except requests.HTTPError:
            print("Request payload:")
            pprint(payload)
            print("Response object:")
            pprint(out)
            raise

        return out

    def version(self):
        return self._rpc("Version", {})["version"]

    def submit_tasks(self, tasks: list[TaskRequest]) -> list[TaskID]:
        requests = []

        for t in tasks:
            task_request = {
                "module": dataclasses.asdict(t.module),
            }
            task_request.update(t.input.serialize())
            requests.append(task_request)
        
        submit_task_request = {
            "requests": requests
        }

        out = self._rpc("SubmitTasks", submit_task_request)
        return out["taskId"]

    def lookup_tasks(self, tasks: list[TaskID]):
        lookup_tasks_request = {
            "taskId": tasks,
        }
        out = self._rpc("LookupTasks", lookup_tasks_request)["responses"]

        responses = []
        for r in out:
            remap(r, "state", "state", TaskState)
            remap(r, "errorMessage", "error")
            remap(r, "processId", "processID", ProcessID)
            remap(r, "httpResponse", "output", HTTPResponse.deserialize)
            remap(r, "taskId", "taskID", TaskID)
            responses.append(TaskResponse(**r))
        return responses