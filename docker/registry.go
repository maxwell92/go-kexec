package docker

// At the time of developing docker registry handling, ie, pushing
// created function to the registry, there is a bug that is not
// resolved in the master branch of "github.com/docker/docker".
// The bug can be found here:
//    https://github.com/docker/docker/issues/26781
//
// The following is just a hack, simply calling command line from
// golang code.
//
// After the community resolve the mentioned bug, the implementation
// should be replaced with real go code. At the meantime, all the
// dev regarding this issue can be found in branch "registry-test".
// Do `git chechout registry-test` to check it out.
//
// Author: Xuan Tang
