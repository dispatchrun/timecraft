import unittest
import os

import timecraft


class TestTimecraft(unittest.TestCase):
    def setUp(self):
        self.client = timecraft.Client()

    def test_version(self):
        version = self.client.version()
        self.assertTrue(len(version) > 0)

    def test_process_id(self):
        process_id = self.client.process_id()
        self.assertTrue(len(process_id) > 0)

    def test_tasks(self):
        process_id = self.client.process_id()

        dir = os.path.dirname(__file__)
        worker_py = os.path.join(dir, "worker.py")
        worker = timecraft.ModuleSpec(path="", args=[worker_py])

        requests = [
            timecraft.HTTPRequest(method="POST",
                                  path="/foo",
                                  body=b"foo",
                                  headers={"X-Foo": "bar"}),
            timecraft.HTTPRequest(method="POST",
                                  path="/bar",
                                  body=b"bar",
                                  headers={"X-Foo": "bar"}),
        ]

        task_requests = [timecraft.TaskRequest(module=worker, input=r)
                         for r in requests]

        task_ids = self.client.submit_tasks(task_requests)

        tasks = self.client.poll_tasks(len(requests), -1)

        self.assertEqual(len(task_ids), len(tasks))

        requests_by_id = dict(zip(task_ids, requests))

        for task in tasks:
            self.assertEqual(task.state, timecraft.TaskState.SUCCESS)

            request = requests_by_id[task.id]
            response = task.output
            headers = response.headers
            self.assertEqual(response.status_code, 200)
            self.assertEqual(response.body, request.body)
            self.assertEqual(headers["X-Timecraft-Task"], task.id)
            self.assertEqual(headers["X-Timecraft-Creator"], process_id)

        tasks = self.client.discard_tasks(task_ids)
