# Golang-Challenge
Challenge test

We ask that you complete the following challenge to evaluate your development skills.

## The Challenge
Finish the implementation of the provided Transparent Cache package.

## Show your work

1.  Create a **Private** repository and share it with the recruiter ( please dont make a pull request, clone the private repository and create a new private one on your profile)
2.  Commit each step of your process so we can follow your thought process.
3.  Give your interviewer access to the private repo

## What to build
Take a look at the current TransparentCache implementation.

You'll see some "TODO" items in the project for features that are still missing.

The solution can be implemented either in Golang or Java ( but you must be able to read code in Golang to realize the exercise ) 

Also, you'll see that some of the provided tests are failing because of that.

The following is expected for solving the challenge:
* Design and implement the missing features in the cache
* Make the failing tests pass, trying to make none (or minimal) changes to them
* Add more tests if needed to show that your implementation really works
 
## Deliverables we expect:
* Your code in a private Github repo
* README file with the decisions taken and important notes

## Time Spent
We suggest not to spend more than 2 hours total, which can be done over the course of 2 days.  Please make commits as often as possible so we can see the time you spent and please do not make one commit.  We will evaluate the code and time spent.
 
What we want to see is how well you handle yourself given the time you spend on the problem, how you think, and how you prioritize when time is insufficient to solve everything.

Please email your solution as soon as you have completed the challenge or the time is up.

## About This Solution

To make this cache safe to read and write in the desired context, the following decisions were made:
    * The cache contains a read-write lock, which was introduced to ensure that cached values can be read and initialized safely. 
    * Each key-value pair has an associated read-write lock to ensure that when a timestamp needs to be updated, this will also be thread safe. This decision was taken in order to allow paralell writes to already initialized values in the cache and improve performance.

Every fetch to the cache will be executed in a separate go-routine. Each routine will communicate the obtained result to the main thread through a separate channel.