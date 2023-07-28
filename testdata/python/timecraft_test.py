import os
import time
import unittest
import socket
import asyncio

import requests
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
        worker_py = os.path.join(dir, "task_worker.py")
        worker = timecraft.ModuleSpec(path="", args=[worker_py])

        requests = [
            timecraft.HTTPRequest(method="POST",
                                  path="/foo",
                                  port=3000,
                                  body=b"foo",
                                  headers={"X-Foo": "bar"}),
            timecraft.HTTPRequest(method="POST",
                                  path="/bar",
                                  port=3000,
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

    def test_spawn(self):
        dir = os.path.dirname(__file__)
        worker_py = os.path.join(dir, "spawn_worker.py")
        worker = timecraft.ModuleSpec(path="", args=[worker_py])

        process_id, ip_address = self.client.spawn(worker)
        try:
            res = None
            for i in range(6):
                try:
                    res = requests.get(f"http://{ip_address}:3000")
                    res.raise_for_status()
                except Exception:
                    pass
                if res is not None and res.ok:
                    break
                time.sleep(0.5)

            self.assertIsNotNone(res)
            self.assertEqual(res.status_code, 200)
        finally:
            self.client.kill(process_id)

    def test_sockpair(self):
        csock, ssock = socket.socketpair()
        csock.sendall(b"42")
        res = ssock.recv(1024)
        csock.close()
        ssock.close()
        self.assertEqual(res, b"42")

    async def sleep(self):
        await asyncio.sleep(1)

    def test_asyncio(self):
        asyncio.run(self.sleep())
        asyncio.run(self.sleep())


if __name__ == "__main__":
    unittest.main()
