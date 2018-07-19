import os
import psutil

def toRelPath(origPath):
    """Converts path to path relative to current script

    origPath:	path to convert
    """
    try:
        if not hasattr(toRelPath, "__location__"):
            toRelPath.__location__ = os.path.realpath(os.path.join(os.getcwd(), os.path.dirname(__file__)))
        return os.path.join(toRelPath.__location__, origPath)
    except NameError:
        return origPath
    
def getMBUsage():
    process = psutil.Process(os.getpid())
    return process.memory_info().rss / 1e6
    
#0: 1080Ti, 1: 940MX
#os.environ["CUDA_VISIBLE_DEVICES"] = "0, 1"