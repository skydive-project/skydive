from setuptools import setup

setup(name='skydive-lib',
      version='0.1',
      description='Skydive library',
      url='http://github.com/skydive-project/skydive',
      author='Skydive Authors',
      author_email='',
      license='Apache2',
      packages=['skydive', 'skydive.rest', 'skydive.websocket'],
      entry_points={
        'console_scripts': [
            'skydive-ws-client = skydive.wsshell:main',
        ],
      },
      zip_safe=False)
