import asyncio
import unittest


class TestAsyncio(unittest.IsolatedAsyncioTestCase):
    async def sleep(self):
        await asyncio.sleep(1)

    def test_asyncio(self):
        asyncio.run(self.sleep())
        asyncio.run(self.sleep())
