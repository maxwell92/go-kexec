package html

const IndexPage = `
<h1>Login</h1>
<form method="post" action="/login">
<label for="name">User name</label>
<input type="text" id="name" name="name">
<label for="password">Password</label>
<input type="password" id="password" name="password">
<button type="submit">Login</button>
</form>
`

const InternalPage = `<!DOCTYPE html>
<html lang="en">
<head>
<title>SymCPE Function-as-a-Service</title>
<style type="text/css" media="screen">
  #editor_div { 
    margin-left: 15px;
    margin-top: 30px;
    width: 1000px;
    height: 400px;
  }

  #wrapper { 
         width: 1000px;
         margin:0 auto;
   }
   
    h1 {
        font:bold 3.5em/0.8em Verdana, Arial, sans-serif;
        color: #555;
        margin-left: 10px;
    }
    
    h2 {
        font: italic 1.1em/0.2em Verdana, Arial, sans-serif;
        color:#666;
        margin-left: 15px;

    }

   #myTextarea {
                position: relative;
                top:430px;
                left:15px;
                width:980px;
                height:100px;
                border-style: solid;
                border-color:#666;
                padding:10px;
                border-radius: 8px;
                font:0.8em Verdana, Arial, sans-serif;
                color:#666;
   }
    
    form  {
         text-align:center;
    }

    button {
          position:relative;
          top: 430px;
          left: 0;
          align-items: flex-start;
          color:#666;
          background-color: #fff;
          width:100px;
          height: 30px;
          border-radius: 8px;
          display: inline-block;
          font-size:14px;
    }

     hr  {
       position: relative;
       top:440px;
       left: 15px;
       color:#666;
     }
     
     .codeuploaded {
                  position: relative;
                  top:430px;
                  left: -430px;
                  font:0.8em Verdana, Arial, sans-serif;
                  color:#666;
     }

</style>
</head>
<body>

<div id="wrapper">

<div id="header">
<h1>Go-Kexec</h1>
<h2>Welcome %s! Thanks for using Go-Kexec</h2>
</div> <!-- this closes header -->

<div id="editor_div">def foo():
    print("ACE Editor is awesome.")

foo()
</div>


<form id="codeForm" action="/create" method="post" enctype="multipart/form-data">
<input type="text" name="functionName" value="default_function">
<select name="runtime">
<option value="python27">Python2.7</option>
</select>
<button type="button" onclick="myFunction()">Submit</button>
<hr>
<p class="codeuploaded">Code Uploaded:</p>
<textarea id="myTextarea" name="codeTextarea" style="display:none;">Default value</textarea>
</form>



<script src="http://d1n0x3qji82z53.cloudfront.net/src-min-noconflict/ace.js" type="text/javascript" charset="utf-8"></script>
<script>

  var editor = ace.edit("editor_div");
  editor.setTheme("ace/theme/monokai");
  editor.getSession().setMode("ace/mode/python");
  
  function myFunction() {
    var textarea = document.getElementById("myTextarea");
    textarea.value = editor.getSession().getValue();
    document.getElementById("codeForm").submit();
    textarea.style.display = "block";
  }

</script>

</div>  <!-- this closes wrapper -->

</body>
</html>
`

const FunctionFailedErrorPage = `
<h1>Function failed. Redirecting...<h1>
%s
`

const FunctionCreatedPage = `
<h1>Function created successfully.<h1>
`

const FunctionCalledPage = `
<h1>Function called successfully.<h1>
`
