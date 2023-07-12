import asyncio


async def main():
    print('Hello ...')
    await asyncio.sleep(1)
    print('... Asyncio :)')


asyncio.run(main())
