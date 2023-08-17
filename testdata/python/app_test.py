import unittest
import asyncio

import sys
import asyncio
import pickle

from timecraft.app import App


class TestApp(unittest.IsolatedAsyncioTestCase):
    def test_run(self):
        res = asyncio.run(app.run())
        self.assertEqual(res, 42)

    async def asyncTearDown(self):
        #await app.close()
        pass



app = App()

@app.function(name="add")
def add(a, b):
    return a + b


@app.start()
async def main():
    res = add.call(40, 2)

    while not await res.is_done():
        await asyncio.sleep(0.1)

    resp = await res.result()
    if resp is not None and resp.output is not None:
        return pickle.loads(resp.output.body)


# TODO: can we move __main__ to the SDK?
if __name__ == "__main__":
    asyncio.run(app.run(sys.argv[1:]))
