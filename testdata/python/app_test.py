import asyncio
import unittest
import sys
import pickle

from timecraft import App


app = App()

class TestApp(unittest.TestCase):
    def test_run(self):
        asyncio.run(app.run(sys.argv[1:]))

    @app.start()
    async def main(self):
        res = self.add.call(40, 2)
    
        while not await res.is_done():
            await asyncio.sleep(0.1)
    
        resp = await res.result()
        result = pickle.loads(resp.output.body)
        self.assertEqual(result, 42)


    @app.function(name="add")
    def add(a, b):
        return a + b


if __name__ == "__main__":
    unittest.main()
