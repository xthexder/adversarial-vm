# adversarial-vm
Fighting fork bombs in a visual address space

## Description

This program implements a simple assembly language interpreter that runs using an image as memory.
Each executing program has an X/Y origin in the image, and a spiraling address space around it.

The current seed program performs these operations:
 - Push ~7KB of random data to the stack, favoring red/green/blue colors
 - Copy the current program to a random location in the image, by manipulating the stack pointers
 - Spawn a new execution at the new location
 - Resume pushing random data to the local stack

The resulting behavior is effectively a fork bomb, where programs are fighting for memory (image) space.
As the execution continues, programs will be overwritten with random data and mutate.
The simulation has a limit set for 100 concurrent programs, since too many threads will slow down the rendering.

Some interesting observations that can be made:
- The average color on screen tends to shift over time as programs mutate
- Smaller replicating programs are favored due to their smaller footprint and replication speed.
- Occationally horizontal lines are generated in the image, which is not an easy operation given the instruction set.
  While this is theoretically possible, it may be a bug in the implementation.

## Screenshots
![Image](https://puu.sh/Ey8dx/1ea56f1181.png)

![Image](https://puu.sh/Ey8dV/8b706e522b.png)

![Image](https://puu.sh/Ey8eu/b72da00b46.png)

![Image](https://puu.sh/Ey8fW/819f35aeb4.png)

![Image](https://puu.sh/Ey8gP/8161f1d368.png)
