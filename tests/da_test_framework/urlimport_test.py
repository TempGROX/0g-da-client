import httpimport

# with httpimport.github_repo('Argeric', 'httpimporttest', ref='main'):
#     from grok import blah
#     n = blah.fib(10)
#     print(n)

with httpimport.github_repo('Argeric', '0g-storage-kv', ref='main'):
    from tests.utility import kv
    n = kv.with_prefix('AFC1772936')
    print(n)
