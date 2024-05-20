# =====> demo1
# from urllib.request import urlopen
# u = urlopen('http://localhost:15000/fib.py')
# data = u.read().decode('utf-8')
# print(data)


# =====> demo2
# import imp
# import urllib.request
# import sys
#
# def load_module(url):
#     u = urllib.request.urlopen(url)
#     source = u.read().decode('utf-8')
#     mod = sys.modules.setdefault(url, imp.new_module(url))
#     code = compile(source, url, 'exec')
#     mod.__file__ = url
#     mod.__package__ = ''
#     exec(code, mod.__dict__)
#     return mod
#
# fib = load_module('http://localhost:15000/fib.py')
# a = fib.fib(10)
# print(a)
# spam = load_module('http://localhost:15000/spam.py')
# b = spam.hello('Guido')
# print(b)


# =====> demo3
# import urlimport
# urlimport.install_meta('http://localhost:15000')
# import fib
# import spam
# a = fib.fib(10)
# print(a)
# b = spam.hello('Guido')
# print(b)


# =====> demo4
import urlimport
urlimport.install_path_hook()
import sys
sys.path.append('http://localhost:15000')
import fib
import spam

a = fib.fib(10)
print(a)
b = spam.hello('Guido')
print(b)
