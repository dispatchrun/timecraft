import unittest
import time

import timecraft


class TestClient(unittest.TestCase):
    def setUp(self):
        self.c = timecraft.Client()

    def test_version(self):
        v = self.c.version()
        self.assertEqual(v, "devel")

    def test_submit_tasks_request(self):
        mod = timecraft.ModuleSpec(path="tests/worker.wasm", args=())
        req = timecraft.HTTPRequest(method="GET", path="/foo", headers={}, body=None)
        t = timecraft.TaskRequest(module=mod, input=req)
        taskids = self.c.submit_tasks([t])

        taskid = taskids[0]

        validated = False
        waiting = True
        while waiting:
            tasks = self.c.lookup_tasks(taskids)
            task = tasks[0]
            match task.state:
                case timecraft.TaskState.QUEUED:
                    pass
                case timecraft.TaskState.INITIALIZING:
                    pass
                case timecraft.TaskState.EXECUTING:
                    self.assertTrue(task.processID)
                case timecraft.TaskState.ERROR:
                    raise Exception(f"should not fail: {task}")
                case timecraft.TaskState.SUCCESS:
                    resp = task.output
                    self.assertEqual(resp.status_code, 200)
                    self.assertEqual(resp.headers["Content-Length"], "3")
                    self.assertEqual(resp.headers["Content-Type"], "text/plain; charset=utf-8")
                    self.assertEqual(resp.body, b"OK!")
                    waiting = False
                    validated = True
                    continue
                case _:
                    raise Exception(f"unknown state: {task}")
            time.sleep(250.0/1000.0)
        self.assertTrue(validated)
