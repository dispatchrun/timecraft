import unittest
import asyncio

from .app import app

class TestApp(unittest.IsolatedAsyncioTestCase):
    def test_run(self):
        res = asyncio.run(app.run())
        self.assertEqual(res, 42)
