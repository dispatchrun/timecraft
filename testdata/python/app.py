import sys
import asyncio
import pickle
from timecraft.app import App

app = App()


@app.function(name="add")
def add(a, b):
    return a + b


@app.start()
async def main():
    print("main")
    res = add.call(40, 2)

    while not await res.is_done():
        await asyncio.sleep(0.1)

    resp = await res.result()
    if resp is not None and resp.output is not None:
        return pickle.loads(resp.output.body)


if __name__ == "__main__":
    asyncio.run(app.run(sys.argv[1:]))
